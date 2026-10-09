// Package fake provides test doubles for the awsapi interfaces.
// Each method is backed by an optional function field, so a test sets only the call it cares about; everything else returns a harmless empty response.

package fake

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
)

type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) record(op string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, op)
}

func (r *recorder) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

// EC2 is a fake awsapi.EC2API.
type EC2 struct {
	DescribeInstancesFunc   func(context.Context, *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeVolumesFunc     func(context.Context, *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error)
	DescribeAddressesFunc   func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error)
	DescribeRegionsFunc     func(context.Context, *ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error)
	DescribeSnapshotsFunc   func(context.Context, *ec2.DescribeSnapshotsInput) (*ec2.DescribeSnapshotsOutput, error)
	DescribeImagesFunc      func(context.Context, *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error)
	DescribeNatGatewaysFunc func(context.Context, *ec2.DescribeNatGatewaysInput) (*ec2.DescribeNatGatewaysOutput, error)

	// Calls records operation names in order, so a test can assert that
	// pagination really made three calls rather than reading one page.
	recorder
}

func (f *EC2) DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.record("DescribeInstances")
	if f.DescribeInstancesFunc != nil {
		return f.DescribeInstancesFunc(ctx, in)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (f *EC2) DescribeVolumes(ctx context.Context, in *ec2.DescribeVolumesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	f.record("DescribeVolumes")
	if f.DescribeVolumesFunc != nil {
		return f.DescribeVolumesFunc(ctx, in)
	}
	return &ec2.DescribeVolumesOutput{}, nil
}

func (f *EC2) DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	f.record("DescribeAddresses")
	if f.DescribeAddressesFunc != nil {
		return f.DescribeAddressesFunc(ctx, in)
	}
	return &ec2.DescribeAddressesOutput{}, nil
}

func (f *EC2) DescribeRegions(ctx context.Context, in *ec2.DescribeRegionsInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	f.record("DescribeRegions")
	if f.DescribeRegionsFunc != nil {
		return f.DescribeRegionsFunc(ctx, in)
	}
	return &ec2.DescribeRegionsOutput{}, nil
}

func (f *EC2) DescribeSnapshots(ctx context.Context, in *ec2.DescribeSnapshotsInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	f.record("DescribeSnapshots")
	if f.DescribeSnapshotsFunc != nil {
		return f.DescribeSnapshotsFunc(ctx, in)
	}
	return &ec2.DescribeSnapshotsOutput{}, nil
}

func (f *EC2) DescribeImages(ctx context.Context, in *ec2.DescribeImagesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	f.record("DescribeImages")
	if f.DescribeImagesFunc != nil {
		return f.DescribeImagesFunc(ctx, in)
	}
	return &ec2.DescribeImagesOutput{}, nil
}

func (f *EC2) DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	f.record("DescribeNatGateways")
	if f.DescribeNatGatewaysFunc != nil {
		return f.DescribeNatGatewaysFunc(ctx, in)
	}
	return &ec2.DescribeNatGatewaysOutput{}, nil
}

// STS is a fake awsapi.STSAPI.
type STS struct {
	GetCallerIdentityFunc func(context.Context, *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
	recorder
}

func (f *STS) GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput,
	_ ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	f.record("GetCallerIdentity")
	if f.GetCallerIdentityFunc != nil {
		return f.GetCallerIdentityFunc(ctx, in)
	}
	account := "123456789012"
	return &sts.GetCallerIdentityOutput{Account: &account}, nil
}

type CloudWatch struct {
	GetMetricDataFunc func(context.Context, *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error)
	recorder
}

func (f *CloudWatch) GetMetricData(ctx context.Context, in *cloudwatch.GetMetricDataInput,
	_ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	f.record("GetMetricData")
	if f.GetMetricDataFunc != nil {
		return f.GetMetricDataFunc(ctx, in)
	}
	return &cloudwatch.GetMetricDataOutput{}, nil
}

// ELB is a fake awsapi.ELBAPI.
type ELB struct {
	DescribeLoadBalancersFunc func(context.Context, *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error)
	DescribeTagsFunc          func(context.Context, *elb.DescribeTagsInput) (*elb.DescribeTagsOutput, error)
	recorder
}

func (f *ELB) DescribeLoadBalancers(ctx context.Context, in *elb.DescribeLoadBalancersInput,
	_ ...func(*elb.Options)) (*elb.DescribeLoadBalancersOutput, error) {
	f.record("DescribeLoadBalancers")
	if f.DescribeLoadBalancersFunc != nil {
		return f.DescribeLoadBalancersFunc(ctx, in)
	}
	return &elb.DescribeLoadBalancersOutput{}, nil
}

func (f *ELB) DescribeTags(ctx context.Context, in *elb.DescribeTagsInput,
	_ ...func(*elb.Options)) (*elb.DescribeTagsOutput, error) {
	f.record("DescribeTags")
	if f.DescribeTagsFunc != nil {
		return f.DescribeTagsFunc(ctx, in)
	}
	return &elb.DescribeTagsOutput{}, nil
}

// RDS is a fake awsapi.RDSAPI.
type RDS struct {
	DescribeDBInstancesFunc func(context.Context, *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error)
	recorder
	DescribeDBSnapshotsFunc func(context.Context, *rds.DescribeDBSnapshotsInput) (*rds.DescribeDBSnapshotsOutput, error)
}

func (f *RDS) DescribeDBInstances(ctx context.Context, in *rds.DescribeDBInstancesInput,
	_ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	f.record("DescribeDBInstances")
	if f.DescribeDBInstancesFunc != nil {
		return f.DescribeDBInstancesFunc(ctx, in)
	}
	return &rds.DescribeDBInstancesOutput{}, nil
}

func (f *RDS) DescribeDBSnapshots(ctx context.Context, in *rds.DescribeDBSnapshotsInput,
	_ ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	f.record("DescribeDBSnapshots")
	if f.DescribeDBSnapshotsFunc != nil {
		return f.DescribeDBSnapshotsFunc(ctx, in)
	}
	return &rds.DescribeDBSnapshotsOutput{}, nil
}

type EFS struct {
	DescribeFileSystemsFunc func(context.Context, *efs.DescribeFileSystemsInput) (*efs.DescribeFileSystemsOutput, error)
	recorder
}

func (f *EFS) DescribeFileSystems(ctx context.Context, in *efs.DescribeFileSystemsInput,
	_ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	f.record("DescribeFileSystems")
	if f.DescribeFileSystemsFunc != nil {
		return f.DescribeFileSystemsFunc(ctx, in)
	}
	return &efs.DescribeFileSystemsOutput{}, nil
}

// Factory is a fake awsapi.Factory.
type Factory struct {
	Clients   awsapi.Clients
	Account   string
	RegionsIn []string
	Base      string

	ForErr     error
	AccountErr error
	RegionsErr error
}

func (f *Factory) For(context.Context, string) (awsapi.Clients, error) {
	return f.Clients, f.ForErr
}

func (f *Factory) AccountID(context.Context) (string, error) {
	if f.AccountErr != nil {
		return "", f.AccountErr
	}
	if f.Account == "" {
		return "123456789012", nil
	}
	return f.Account, nil
}

func (f *Factory) Regions(context.Context) ([]string, error) {
	return f.RegionsIn, f.RegionsErr
}

func (f *Factory) BaseRegion() string {
	if f.Base == "" {
		return "us-east-1"
	}
	return f.Base
}

// Compile-time proof the fakes still match the real interfaces.
var (
	_ awsapi.EC2API        = (*EC2)(nil)
	_ awsapi.STSAPI        = (*STS)(nil)
	_ awsapi.ELBAPI        = (*ELB)(nil)
	_ awsapi.RDSAPI        = (*RDS)(nil)
	_ awsapi.CloudWatchAPI = (*CloudWatch)(nil)
	_ awsapi.Factory       = (*Factory)(nil)
	_ awsapi.EFSAPI        = (*EFS)(nil)
)
