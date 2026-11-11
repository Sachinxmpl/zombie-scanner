package test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// an account with one zombie for every check and fakes that remember every call
type world struct {
	ec2 *fake.EC2
	cw  *fake.CloudWatch
	elb *fake.ELB
	rds *fake.RDS
	efs *fake.EFS
	kms *fake.KMS
	ecr *fake.ECR
}

func newWorld() *world {
	old := aws.Time(daysAgo(400))
	return &world{
		ec2: &fake.EC2{
			DescribeVolumesFunc: func(context.Context, *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
				return &ec2.DescribeVolumesOutput{Volumes: []ec2types.Volume{
					{VolumeId: aws.String("vol-free"), State: ec2types.VolumeStateAvailable, VolumeType: ec2types.VolumeTypeGp3, Size: aws.Int32(100), CreateTime: old},
					{VolumeId: aws.String("vol-root"), State: ec2types.VolumeStateInUse, VolumeType: ec2types.VolumeTypeGp3, Size: aws.Int32(8), CreateTime: old,
						Attachments: []ec2types.VolumeAttachment{{InstanceId: aws.String("i-stopped")}}},
				}}, nil
			},
			DescribeInstancesFunc: func(context.Context, *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
				return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{
					InstanceId: aws.String("i-stopped"), InstanceType: ec2types.InstanceTypeT3Micro, LaunchTime: old,
					State:                 &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
					StateTransitionReason: aws.String("User initiated (2025-01-01 00:00:00 GMT)"),
					BlockDeviceMappings:   []ec2types.InstanceBlockDeviceMapping{{Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-root")}}},
				}}}}}, nil
			},
			DescribeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
				return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{
					{AllocationId: aws.String("eipalloc-free"), PublicIp: aws.String("1.1.1.1")},
					{AllocationId: aws.String("eipalloc-spare"), PublicIp: aws.String("4.4.4.4"), AssociationId: aws.String("a-1"), NetworkInterfaceId: aws.String("eni-spare")},
					// decoy: a NAT gateway's interface is in use
					{AllocationId: aws.String("eipalloc-nat"), PublicIp: aws.String("3.3.3.3"), AssociationId: aws.String("a-2"), NetworkInterfaceId: aws.String("eni-nat")},
				}}, nil
			},
			DescribeNetworkInterfacesFunc: interfacesAPI(map[string]bool{"eni-spare": true}, nil),
			DescribeSnapshotsFunc: func(context.Context, *ec2.DescribeSnapshotsInput) (*ec2.DescribeSnapshotsOutput, error) {
				return &ec2.DescribeSnapshotsOutput{Snapshots: []ec2types.Snapshot{
					{SnapshotId: aws.String("snap-old"), VolumeSize: aws.Int32(50), StartTime: old},
					{SnapshotId: aws.String("snap-ami"), VolumeSize: aws.Int32(50), StartTime: old},
				}}, nil
			},
			// decoy: an AMI still uses snap-ami
			DescribeImagesFunc: func(context.Context, *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
				return &ec2.DescribeImagesOutput{Images: []ec2types.Image{{ImageId: aws.String("ami-1"),
					BlockDeviceMappings: []ec2types.BlockDeviceMapping{{Ebs: &ec2types.EbsBlockDevice{SnapshotId: aws.String("snap-ami")}}}}}}, nil
			},
			DescribeNatGatewaysFunc: func(context.Context, *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error) {
				return &ec2.DescribeNatGatewaysOutput{NatGateways: []ec2types.NatGateway{
					{NatGatewayId: aws.String("nat-idle"), State: ec2types.NatGatewayStateAvailable, CreateTime: old},
				}}, nil
			},
		},
		// every metric reads zero, so everything with metrics looks idle
		cw: &fake.CloudWatch{GetMetricDataFunc: func(_ context.Context, in *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
			out := &cloudwatch.GetMetricDataOutput{}
			for _, q := range in.MetricDataQueries {
				out.MetricDataResults = append(out.MetricDataResults, cwtypes.MetricDataResult{Id: q.Id, Values: []float64{0}})
			}
			return out, nil
		}},
		elb: &fake.ELB{DescribeLoadBalancersFunc: func(context.Context, *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error) {
			return &elb.DescribeLoadBalancersOutput{LoadBalancers: []elbtypes.LoadBalancer{{
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/web/50dc6c495c0c9188"),
				LoadBalancerName: aws.String("web"), Type: elbtypes.LoadBalancerTypeEnumApplication, CreatedTime: old,
			}}}, nil
		}},
		rds: &fake.RDS{
			DescribeDBInstancesFunc: func(context.Context, *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
				return &rds.DescribeDBInstancesOutput{DBInstances: []rdstypes.DBInstance{
					{DBInstanceIdentifier: aws.String("db-stopped"), DBInstanceStatus: aws.String("stopped"), Engine: aws.String("postgres"),
						DBInstanceClass: aws.String("db.t3.micro"), StorageType: aws.String("gp3"), AllocatedStorage: aws.Int32(20), InstanceCreateTime: old},
					{DBInstanceIdentifier: aws.String("db-idle"), DBInstanceStatus: aws.String("available"), Engine: aws.String("postgres"),
						DBInstanceClass: aws.String("db.t3.micro"), StorageType: aws.String("gp3"), AllocatedStorage: aws.Int32(20), InstanceCreateTime: old},
				}}, nil
			},
			DescribeDBSnapshotsFunc: func(context.Context, *rds.DescribeDBSnapshotsInput) (*rds.DescribeDBSnapshotsOutput, error) {
				return &rds.DescribeDBSnapshotsOutput{DBSnapshots: []rdstypes.DBSnapshot{
					{DBSnapshotIdentifier: aws.String("dbsnap-old"), SnapshotType: aws.String("manual"), Status: aws.String("available"),
						AllocatedStorage: aws.Int32(20), SnapshotCreateTime: old},
				}}, nil
			},
		},
		efs: &fake.EFS{DescribeFileSystemsFunc: func(context.Context, *efs.DescribeFileSystemsInput) (*efs.DescribeFileSystemsOutput, error) {
			return &efs.DescribeFileSystemsOutput{FileSystems: []efstypes.FileSystemDescription{{
				FileSystemId: aws.String("fs-orphan"), LifeCycleState: efstypes.LifeCycleStateAvailable, NumberOfMountTargets: 0,
				CreationTime: old, SizeInBytes: &efstypes.FileSystemSize{Value: 5 << 30, ValueInStandard: aws.Int64(5 << 30)},
			}}}, nil
		}},
		kms: kmsWithTags(func(string) ([]kmstypes.Tag, error) { return nil, nil }),
		ecr: ecrWithRepoTags(func(string) ([]ecrtypes.Tag, error) { return nil, nil }),
	}
}

func (w *world) engine() *scan.Engine {
	return &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: w.ec2, CW: w.cw, ELB: w.elb, RDS: w.rds, EFS: w.efs, KMS: w.kms, ECR: w.ecr},
			Base:    "us-east-1",
		}},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}
}

// every call made, as "service:Operation"
func (w *world) calls() []string {
	out := []string{}
	add := func(service string, ops []string) {
		for _, op := range ops {
			out = append(out, service+":"+op)
		}
	}
	add("ec2", w.ec2.Calls())
	add("cloudwatch", w.cw.Calls())
	add("elasticloadbalancing", w.elb.Calls())
	add("rds", w.rds.Calls())
	add("elasticfilesystem", w.efs.Calls())
	add("kms", w.kms.Calls())
	add("ecr", w.ecr.Calls())
	return out
}

func byDetector(fs []zombie.Finding) map[string][]string {
	out := map[string][]string{}
	for _, f := range fs {
		out[f.Detector] = append(out[f.Detector], f.ResourceID)
	}
	return out
}

// Running one check must find exactly what it finds in a full scan, and call only what it needs.
func TestEachCheckAloneMatchesFullScan(t *testing.T) {
	full, err := newWorld().engine().Run(context.Background(), scan.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := byDetector(full.Findings)
	// pinned, so data lost in the full scan itself is caught too
	pinned := map[string][]string{
		"ebs-unattached":    {"vol-free"},
		"ecr-untagged":      {"sha256:kept1", "sha256:kept2", "sha256:plain1", "sha256:plain2"},
		"efs-unused":        {"fs-orphan"},
		"eip-unassociated":  {"eipalloc-free", "eipalloc-spare"},
		"elb-idle":          {"web"},
		"instance-stopped":  {"i-stopped"},
		"kms-key-disabled":  {"key-kept", "key-plain"},
		"nat-idle":          {"nat-idle"},
		"rds-idle":          {"db-idle"},
		"rds-snapshot-aged": {"dbsnap-old"},
		"rds-stopped":       {"db-stopped"},
		"snapshot-aged":     {"snap-old"},
	}
	for name, ids := range want {
		slices.Sort(ids)
		if !slices.Equal(ids, pinned[name]) {
			t.Errorf("full scan: %s found %v, want %v", name, ids, pinned[name])
		}
	}

	for _, d := range detect.All() {
		t.Run(d.Name(), func(t *testing.T) {
			// the world must exercise every check, or this test proves nothing for it
			if len(want[d.Name()]) == 0 {
				t.Fatalf("the test world has no zombie for %s; add one", d.Name())
			}

			w := newWorld()
			rep, err := w.engine().Run(context.Background(), scan.Options{Only: []string{d.Name()}})
			if err != nil {
				t.Fatal(err)
			}
			got := byDetector(rep.Findings)[d.Name()]
			slices.Sort(got)
			if !slices.Equal(got, want[d.Name()]) {
				t.Errorf("alone found %v, full scan found %v", got, want[d.Name()])
			}
			for _, c := range w.calls() {
				if !slices.Contains(d.Needs(), c) {
					t.Errorf("called %s, which %s does not need", c, d.Name())
				}
			}
		})
	}
}

func TestSkipLeavesOutTheirCalls(t *testing.T) {
	skip := []string{}
	for _, d := range detect.All() {
		if d.Name() != "ebs-unattached" {
			skip = append(skip, d.Name())
		}
	}
	w := newWorld()
	if _, err := w.engine().Run(context.Background(), scan.Options{Skip: skip}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(w.calls(), " "); got != "ec2:DescribeVolumes" {
		t.Errorf("calls = %s, want only ec2:DescribeVolumes", got)
	}
}
