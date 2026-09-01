package collect

import (
	"context"
	"fmt"

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
