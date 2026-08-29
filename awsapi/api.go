package awsapi

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// -> aws api is boudary between scanner and  AWS. Exists -> so no other package holds a concrete SDK client

// Only the EC2 operations this tool uses
type EC2API interface {
	DescribeVolumes(ctx context.Context, in *ec2.DescribeVolumesInput, opts ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, opts ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	DescribeRegions(ctx context.Context, in *ec2.DescribeRegionsInput, opts ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
}

// Used once at startup to learn account ID
type STSAPI interface {
	GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// Compile time proof that real SDK clients satisfy these interfaces
// SDK upgrades -> methods changed -> build breaks here
var (
	_ EC2API = (*ec2.Client)(nil)
	_ STSAPI = (*sts.Client)(nil)
)

// one regions's worth of AWS clients
type Clients struct {
	EC2 EC2API
	// ELB, S3
}

type Factory interface {
	// returns clients bound to one region
	For(ctx context.Context, region string) (Clients, error)

	// sts.GetCallerIdentity cached
	AccountID(ctx context.Context) (string, error)

	// ec2:DescribeRegions, cached -> regions this account has opted into
	Regions(ctx context.Context) ([]string, error)

	BaseRegion() string
}
