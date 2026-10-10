package awsapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// just two, everything else comes from standard aws credential chain
type Options struct {
	Profile       string // --profile
	Region        string // --region
	AssumeRoleARN string
}

type factory struct {
	base aws.Config

	mu      sync.Mutex
	clients map[string]Clients

	accountId string
	regions   []string

	roleARN string
}

func New(ctx context.Context, o Options) (Factory, error) {
	loadOpts := []func(*config.LoadOptions) error{
		// SDK already implements backoff, jitter, client-side rate limiter (learns from throttle response)
		config.WithRetryMode(aws.RetryModeAdaptive),
	}

	if o.Profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(o.Profile))
	}

	if o.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(o.Region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	if cfg.Region == "" {
		return nil, errors.New(
			"no aws region configured: set --region, AWS_REGION or a region in ~/.aws/config",
		)
	}

	if o.AssumeRoleARN != "" {
		cfg.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(
				sts.NewFromConfig(cfg),
				o.AssumeRoleARN,
				func(p *stscreds.AssumeRoleOptions) {
					p.RoleSessionName = "zombie-scanner"
				},
			),
		)
	}

	return &factory{base: cfg, roleARN: o.AssumeRoleARN, clients: make(map[string]Clients)}, nil
}

func (f *factory) BaseRegion() string {
	return f.base.Region
}

// Returns clients for one region, building them once and caching them
// clients -> concurrent safe
func (f *factory) For(_ context.Context, region string) (Clients, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if c, ok := f.clients[region]; ok {
		return c, nil
	}
	cfg := f.base.Copy()
	cfg.Region = region

	c := Clients{
		EC2: ec2.NewFromConfig(cfg),
		CW:  cloudwatch.NewFromConfig(cfg),
		ELB: elb.NewFromConfig(cfg),
		RDS: rds.NewFromConfig(cfg),
		EFS: efs.NewFromConfig(cfg),
		KMS: kms.NewFromConfig(cfg),
		ECR: ecr.NewFromConfig(cfg),
	}
	f.clients[region] = c
	return c, nil
}

// calls sts:GetCallerIdentity once and caches the result
func (f *factory) AccountID(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.accountId != "" {
		return f.accountId, nil
	}

	out, err := sts.NewFromConfig(f.base).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		if f.roleARN != "" {
			return "", fmt.Errorf("sts:GetCallerIdentity (role %s): %w", f.roleARN, err)
		}
		return "", fmt.Errorf("sts:GetCallerIdentity: %w", err)
	}
	f.accountId = aws.ToString(out.Account)
	return f.accountId, nil
}

// returns regions this account has opted into
func (f *factory) Regions(ctx context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.regions != nil {
		return f.regions, nil
	}

	out, err := ec2.NewFromConfig(f.base).DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("ec2:DescribeRegions: %w", err)
	}

	names := make([]string, 0, len(out.Regions))
	for _, r := range out.Regions {
		if n := aws.ToString(r.RegionName); n != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	f.regions = names

	return f.regions, nil
}
