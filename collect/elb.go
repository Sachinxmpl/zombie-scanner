package collect

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// Returns every ELBv2 load balancer, application and network.
// Classic load balancers use a different API. currently our of scope
func LoadBalancers(ctx context.Context, api awsapi.ELBAPI) ([]zombie.LoadBalancer, error) {
	out := []zombie.LoadBalancer{}

	p := elb.NewDescribeLoadBalancersPaginator(api, &elb.DescribeLoadBalancersInput{})

	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("elasticloadbalancing:DescribeLoadBalancers: %w", err)
		}
		for _, lb := range page.LoadBalancers {
			out = append(out, toLoadBalancer(lb))
		}
	}
	return out, nil
}

const tagsPerCall = 20

// Fills in Tags on Load Balancers
// DescribeLoadBalancers doesnot return tags, so DescribeTags is used
func LoadBalancerTags(ctx context.Context, api awsapi.ELBAPI, lbs []zombie.LoadBalancer) error {
	byARN := make(map[string]int, len(lbs))
	arns := make([]string, 0, len(lbs))
	for i, lb := range lbs {
		if lb.ARN == "" {
			continue
		}
		byARN[lb.ARN] = i
		arns = append(arns, lb.ARN)
	}

	for base := 0; base < len(arns); base += tagsPerCall {
		batch := arns[base:min(base+tagsPerCall, len(arns))]

		resp, err := api.DescribeTags(ctx, &elb.DescribeTagsInput{ResourceArns: batch})
		if err != nil {
			return fmt.Errorf("elasticloadbalancing:DescribeTags: %w", err)
		}
		for _, td := range resp.TagDescriptions {
			i, ok := byARN[aws.ToString(td.ResourceArn)]
			if !ok {
				continue
			}
			lbs[i].Tags = toELBTags(td.Tags)
		}
	}

	return nil
}
