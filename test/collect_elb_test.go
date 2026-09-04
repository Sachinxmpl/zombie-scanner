package test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/collect"
)

// CloudWatch's LoadBalancer dimension wants "app/my-alb/50dc...", not the ARN.
// Passing the ARN returns zero datapoints, which looks exactly like "idle".
func TestLoadBalancerMetricSuffixIsNotTheARN(t *testing.T) {
	const arn = "arn:aws:elasticloadbalancing:us-east-1:520646130605:loadbalancer/app/my-alb/50dc6c495c0c9188"

	f := &fake.ELB{
		DescribeLoadBalancersFunc: func(context.Context, *elb.DescribeLoadBalancersInput) (*elb.DescribeLoadBalancersOutput, error) {
			return &elb.DescribeLoadBalancersOutput{LoadBalancers: []elbtypes.LoadBalancer{
				{
					LoadBalancerArn:  aws.String(arn),
					LoadBalancerName: aws.String("my-alb"),
					Type:             elbtypes.LoadBalancerTypeEnumApplication,
				},
				{
					LoadBalancerArn:  aws.String("malformed"),
					LoadBalancerName: aws.String("broken"),
				},
			}}, nil
		},
	}

	got, err := collect.LoadBalancers(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d load balancers, want 2", len(got))
	}

	if got[0].MetricSuffix != "app/my-alb/50dc6c495c0c9188" {
		t.Errorf("MetricSuffix = %q", got[0].MetricSuffix)
	}
	if got[0].MetricSuffix == got[0].ARN {
		t.Error("suffix is the ARN - CloudWatch would return no datapoints")
	}

	// a malformed ARN must yield no dimension value, so the detector skips it
	if got[1].MetricSuffix != "" {
		t.Errorf("malformed ARN produced suffix %q, want empty", got[1].MetricSuffix)
	}
}
