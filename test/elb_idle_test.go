package test

import (
	"testing"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func TestELBIdleSkipsWhenTagsCouldNotBeRead(t *testing.T) {
	lb := zombie.LoadBalancer{
		ARN:          "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/web/50dc6c495c0c9188",
		Name:         "web",
		Type:         "application",
		MetricSuffix: "app/web/50dc6c495c0c9188",
		CreatedAt:    daysAgo(100),
	}
	cfg := detect.Defaults()
	metrics := zombie.NewMetricSet(time.Duration(cfg.IdleWindowDays) * 24 * time.Hour)
	metrics.Set(zombie.MetricKey{NameSpace: "AWS/ApplicationELB", Metric: "RequestCount", ResourceID: lb.MetricSuffix}, 0)

	for _, tc := range []struct {
		name   string
		failed map[string]bool
		want   int
	}{
		{"tags read", nil, 1},
		{"tags could not be read", map[string]bool{"elasticloadbalancing:DescribeTags": true}, 0},
		{"an unrelated call failed", map[string]bool{"ec2:DescribeVolumes": true}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := zombie.Inventory{
				Now:           now,
				LoadBalancers: []zombie.LoadBalancer{lb},
				Metrics:       metrics,
				Failed:        tc.failed,
			}
			got := detect.Run(inv, cfg, []string{"elb-idle"}, nil)
			if len(got) != tc.want {
				t.Errorf("got %d findings, want %d", len(got), tc.want)
			}
		})
	}
}
