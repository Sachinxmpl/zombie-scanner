// Package fake provides test doubles for the awsapi interfaces.
// Each method is backed by an optional function field, so a test sets only the call it cares about; everything else returns a harmless empty response.

package fake

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
)

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
	Calls []string
}

func (f *EC2) DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.Calls = append(f.Calls, "DescribeInstances")
	if f.DescribeInstancesFunc != nil {
		return f.DescribeInstancesFunc(ctx, in)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (f *EC2) DescribeVolumes(ctx context.Context, in *ec2.DescribeVolumesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	f.Calls = append(f.Calls, "DescribeVolumes")
	if f.DescribeVolumesFunc != nil {
		return f.DescribeVolumesFunc(ctx, in)
	}
	return &ec2.DescribeVolumesOutput{}, nil
}

func (f *EC2) DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	f.Calls = append(f.Calls, "DescribeAddresses")
	if f.DescribeAddressesFunc != nil {
		return f.DescribeAddressesFunc(ctx, in)
	}
	return &ec2.DescribeAddressesOutput{}, nil
}

func (f *EC2) DescribeRegions(ctx context.Context, in *ec2.DescribeRegionsInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error) {
	f.Calls = append(f.Calls, "DescribeRegions")
	if f.DescribeRegionsFunc != nil {
		return f.DescribeRegionsFunc(ctx, in)
	}
	return &ec2.DescribeRegionsOutput{}, nil
}

func (f *EC2) DescribeSnapshots(ctx context.Context, in *ec2.DescribeSnapshotsInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	f.Calls = append(f.Calls, "DescribeSnapshots")
	if f.DescribeSnapshotsFunc != nil {
		return f.DescribeSnapshotsFunc(ctx, in)
	}
	return &ec2.DescribeSnapshotsOutput{}, nil
}

func (f *EC2) DescribeImages(ctx context.Context, in *ec2.DescribeImagesInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	f.Calls = append(f.Calls, "DescribeImages")
	if f.DescribeImagesFunc != nil {
		return f.DescribeImagesFunc(ctx, in)
	}
	return &ec2.DescribeImagesOutput{}, nil
}

func (f *EC2) DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput,
	_ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	f.Calls = append(f.Calls, "DescribeNatGateways")
	if f.DescribeNatGatewaysFunc != nil {
		return f.DescribeNatGatewaysFunc(ctx, in)
	}
	return &ec2.DescribeNatGatewaysOutput{}, nil
}

// STS is a fake awsapi.STSAPI.
type STS struct {
	GetCallerIdentityFunc func(context.Context, *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
	Calls                 []string
}

func (f *STS) GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput,
	_ ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	f.Calls = append(f.Calls, "GetCallerIdentity")
	if f.GetCallerIdentityFunc != nil {
		return f.GetCallerIdentityFunc(ctx, in)
	}
	account := "123456789012"
	return &sts.GetCallerIdentityOutput{Account: &account}, nil
}

type CloudWatch struct {
	GetMetricDataFunc func(context.Context, *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error)
	Calls             []string
}

func (f *CloudWatch) GetMetricData(ctx context.Context, in *cloudwatch.GetMetricDataInput,
	_ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	f.Calls = append(f.Calls, "GetMetricData")
	if f.GetMetricDataFunc != nil {
		return f.GetMetricDataFunc(ctx, in)
	}
	return &cloudwatch.GetMetricDataOutput{}, nil
}

// ELB is a fake awsapi.ELBAPI.
type ELB struct {
	DescribeLoadBalancersFunc func(context.Context, *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error)
	Calls                     []string
}

func (f *ELB) DescribeLoadBalancers(ctx context.Context, in *elb.DescribeLoadBalancersInput,
	_ ...func(*elb.Options)) (*elb.DescribeLoadBalancersOutput, error) {
	f.Calls = append(f.Calls, "DescribeLoadBalancers")
	if f.DescribeLoadBalancersFunc != nil {
		return f.DescribeLoadBalancersFunc(ctx, in)
	}
	return &elb.DescribeLoadBalancersOutput{}, nil
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
	_ awsapi.EC2API  = (*EC2)(nil)
	_ awsapi.STSAPI  = (*STS)(nil)
	_ awsapi.ELBAPI  = (*ELB)(nil)
	_ awsapi.Factory = (*Factory)(nil)
)
