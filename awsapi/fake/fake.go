// Package fake provides test doubles for the awsapi interfaces.
// Each method is backed by an optional function field, so a test sets only the call it cares about; everything else returns a harmless empty response.

package fake

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
)

// EC2 is a fake awsapi.EC2API.
type EC2 struct {
	DescribeVolumesFunc   func(context.Context, *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error)
	DescribeAddressesFunc func(context.Context, *ec2.DescribeAddressesInput) (*ec2.DescribeAddressesOutput, error)
	DescribeRegionsFunc   func(context.Context, *ec2.DescribeRegionsInput) (*ec2.DescribeRegionsOutput, error)

	// Calls records operation names in order, so a test can assert that
	// pagination really made three calls rather than reading one page.
	Calls []string
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
	_ awsapi.Factory = (*Factory)(nil)
)
