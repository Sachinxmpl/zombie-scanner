package detect

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(elbIdle{})
}

type elbIdle struct{}

func (elbIdle) Name() string {
	return "elb-idle"
}

func (elbIdle) Describe() string {
	return "Application load balancers that served almost no requests over the metric window"
}

func (elbIdle) Needs() []string {
	return []string{
		"elasticloadbalancing:DescribeLoadBalancers",
		"cloudwatch:GetMetricData",
	}
}

func (elbIdle) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}
	window := time.Duration(cfg.IdleWindowDays) * 24 * time.Hour

	for _, lb := range inv.LoadBalancers {
		// RequestCount does not exist for network load balancers
		if lb.Type != "application" {
			continue
		}

		// an unparseable ARN left us no dimension value to query
		if lb.MetricSuffix == "" {
			continue
		}

		// no history, no verdict
		if inv.Now.Sub(lb.CreatedAt) < window {
			continue
		}

		sum, ok := inv.Metrics.Sum(zombie.MetricKey{
			NameSpace:  "AWS/ApplicationELB",
			Metric:     "RequestCount",
			ResourceID: lb.MetricSuffix,
		})
		// missing data is unknown, never idle
		if !ok {
			continue
		}
		if sum >= cfg.ELBIdleRequests {
			continue
		}

		created := lb.CreatedAt
		f := zombie.Finding{
			ResourceID:   lb.Name,
			ResourceType: "alb",
			ResourceARN:  lb.ARN,
			Confidence:   zombie.Medium,
			Reason: fmt.Sprintf("%.0f requests over %d days (checked %s)",
				sum, cfg.IdleWindowDays, inv.Now.Format("2006-01-02")),
			CreatedAt: &created,
			Tags:      lb.Tags,
		}
		f.Meta("requests", strconv.FormatFloat(sum, 'f', 0, 64))
		f.Meta("window_days", strconv.Itoa(cfg.IdleWindowDays))
		f.Meta("metric_suffix", lb.MetricSuffix)

		out = append(out, f)
	}

	return out
}
