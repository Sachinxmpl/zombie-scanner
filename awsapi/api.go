package awsapi

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// -> aws api is boudary between scanner and  AWS. Exists -> so no other package holds a concrete SDK client

// Only the EC2 operations this tool uses
type EC2API interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeVolumes(ctx context.Context, in *ec2.DescribeVolumesInput, opts ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, opts ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	DescribeRegions(ctx context.Context, in *ec2.DescribeRegionsInput, opts ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
	DescribeSnapshots(ctx context.Context, in *ec2.DescribeSnapshotsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	DescribeImages(ctx context.Context, in *ec2.DescribeImagesInput, opts ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput, opts ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
}

// Used once at startup to learn account ID
type STSAPI interface {
	GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type CloudWatchAPI interface {
	GetMetricData(ctx context.Context, in *cloudwatch.GetMetricDataInput, opts ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

// Compile time proof that real SDK clients satisfy these interfaces
// SDK upgrades -> methods changed -> build breaks here
var (
	_ EC2API        = (*ec2.Client)(nil)
	_ STSAPI        = (*sts.Client)(nil)
	_ CloudWatchAPI = (*cloudwatch.Client)(nil)
	_ ELBAPI        = (*elb.Client)(nil)
)

// one regions's worth of AWS clients
// Only the ELBv2 operations this tool uses. Classic load balancers use a
// different API and are out of scope.
type ELBAPI interface {
	DescribeLoadBalancers(ctx context.Context, in *elb.DescribeLoadBalancersInput, opts ...func(*elb.Options)) (*elb.DescribeLoadBalancersOutput, error)
}

type Clients struct {
	EC2 EC2API
	CW  CloudWatchAPI
	ELB ELBAPI
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
