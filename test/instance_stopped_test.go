package test

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

const stoppedLongAgo = "User initiated (2026-01-01 00:00:00 GMT)"

func stoppedInventory(volumes []zombie.Volume, addresses []zombie.Address) zombie.Inventory {
	inst := zombie.Instance{ID: "i-1", Type: "t3.micro", State: "stopped",
		StateTransitionReason: stoppedLongAgo}
	for _, v := range volumes {
		inst.VolumeIDs = append(inst.VolumeIDs, v.ID)
	}
	return zombie.Inventory{Now: now, Instances: []zombie.Instance{inst},
		Volumes: volumes, Addresses: addresses}
}

func scanStopped(inv zombie.Inventory, detectors ...string) []zombie.Finding {
	if len(detectors) == 0 {
		detectors = []string{"instance-stopped"}
	}
	return price.Apply(detect.Run(inv, detect.Defaults(), detectors, nil), "us-east-1")
}

// A stopped instance's automatic public IP is released, but an Elastic IP
// stays attached and bills $3.65/month on top of the volumes.
func TestStoppedInstanceCountsItsElasticIPs(t *testing.T) {
	disk := []zombie.Volume{{ID: "vol-1", VolumeType: "gp3", SizeGiB: 8, AttachedTo: "i-1"}}
	onThis := zombie.Address{AllocationID: "eipalloc-1", PublicIP: "203.0.113.1",
		AssociationID: "eipassoc-1", InstanceID: "i-1"}
	onOther := zombie.Address{AllocationID: "eipalloc-2", PublicIP: "203.0.113.2",
		AssociationID: "eipassoc-2", InstanceID: "i-other"}

	for _, tc := range []struct {
		name     string
		inv      zombie.Inventory
		wantCost float64
		wantText string
	}{
		{"volume and its IP", stoppedInventory(disk, []zombie.Address{onThis}), 4.29, "1 Elastic IP"},
		{"another instance's IP", stoppedInventory(disk, []zombie.Address{onOther}), 0.64, "8 GiB gp3"},
		{"IP only, no disks", stoppedInventory(nil, []zombie.Address{onThis}), 3.65, "1 Elastic IP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := scanStopped(tc.inv)
			if len(got) != 1 {
				t.Fatalf("got %d findings, want 1", len(got))
			}
			if math.Abs(got[0].MonthlyCost-tc.wantCost) > 0.001 {
				t.Errorf("cost = %.4f, want %.2f (%s)", got[0].MonthlyCost, tc.wantCost, got[0].CostBasis)
			}
			if !strings.Contains(got[0].CostBasis, tc.wantText) {
				t.Errorf("cost basis %q does not mention %q", got[0].CostBasis, tc.wantText)
			}
		})
	}
}

// A denied DescribeAddresses must not hide the instance or lose its volume cost.
func TestStoppedInstanceWithoutAddressesStillPricesVolumes(t *testing.T) {
	inv := stoppedInventory([]zombie.Volume{{ID: "vol-1", VolumeType: "gp3", SizeGiB: 8, AttachedTo: "i-1"}}, nil)
	inv.Failed = map[string]bool{"ec2:DescribeAddresses": true}

	got := scanStopped(inv)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	if math.Abs(got[0].MonthlyCost-0.64) > 0.001 {
		t.Errorf("cost = %.4f, want 0.64", got[0].MonthlyCost)
	}
	if !strings.Contains(got[0].CostBasis, "not checked") {
		t.Errorf("cost basis %q does not say the IPs were not checked", got[0].CostBasis)
	}
}

// The IP is attached, so eip-unassociated must not report it a second time.
func TestStoppedInstanceIPIsCountedOnce(t *testing.T) {
	inv := stoppedInventory(nil, []zombie.Address{{AllocationID: "eipalloc-1", PublicIP: "203.0.113.1",
		AssociationID: "eipassoc-1", InstanceID: "i-1"}})

	got := scanStopped(inv, "instance-stopped", "eip-unassociated")
	if len(got) != 1 {
		t.Fatalf("got %v, want only the instance", ids(got))
	}
}

// Through the engine, so a broken InstanceId mapping in collect is caught too.
func TestStoppedInstanceElasticIPThroughTheEngine(t *testing.T) {
	ec2api := &fake.EC2{
		DescribeInstancesFunc: func(context.Context, *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: []ec2types.Instance{{
				InstanceId:            aws.String("i-1"),
				InstanceType:          ec2types.InstanceTypeT3Micro,
				State:                 &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
				StateTransitionReason: aws.String(stoppedLongAgo),
			}}}}}, nil
		},
		DescribeAddressesFunc: func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error) {
			return &ec2.DescribeAddressesOutput{Addresses: []ec2types.Address{{
				AllocationId:  aws.String("eipalloc-1"),
				PublicIp:      aws.String("203.0.113.1"),
				AssociationId: aws.String("eipassoc-1"),
				InstanceId:    aws.String("i-1"),
			}}}, nil
		},
	}
	eng := &scan.Engine{
		Accounts: []awsapi.Factory{&fake.Factory{
			Clients: awsapi.Clients{EC2: ec2api, CW: &fake.CloudWatch{}, ELB: &fake.ELB{}, RDS: &fake.RDS{},
				EFS: &fake.EFS{}, KMS: &fake.KMS{}, ECR: &fake.ECR{}},
			Account: "123456789012",
			Base:    "us-east-1",
		}},
		Cfg:   detect.Defaults(),
		Clock: func() time.Time { return now },
	}

	report, err := eng.Run(context.Background(), scan.Options{Only: []string{"instance-stopped"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("got %v, want i-1", ids(report.Findings))
	}
	if math.Abs(report.Findings[0].MonthlyCost-3.65) > 0.001 {
		t.Errorf("cost = %.4f, want 3.65 - the IP never reached the instance", report.Findings[0].MonthlyCost)
	}
}
