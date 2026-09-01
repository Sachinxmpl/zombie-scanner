package detect

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(natIdle{})
}

type natIdle struct{}

func (natIdle) Name() string {
	return "nat-idle"
}

func (natIdle) Describe() string {
	return "NAT gateways with almost no outbound traffic over the metric window."
}

func (natIdle) Needs() []string {
	return []string{
		"ec2:DescribeNatGateways",
		"cloudwatch:GetMetricData",
	}
}

func (natIdle) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}
	window := time.Duration(cfg.IdleWindowDays) * 24 * time.Hour

	for _, n := range inv.NATGateways {
		if n.State != "available" {
			continue
		}

		// no history
		if inv.Now.Sub(n.CreatedAt) < window {
			continue
		}

		sum, ok := inv.Metrics.Sum(zombie.MetricKey{
			NameSpace:  "AWS/NATGateway",
			Metric:     "BytesOutToDestination",
			ResourceID: n.ID,
		})
		// missing data -> treat as unknown, not idle
		if !ok {
			continue
		}
		if sum >= cfg.NATIdleBytes {
			continue
		}

		created := n.CreatedAt
		f := zombie.Finding{
			ResourceID:   n.ID,
			ResourceType: "nat-gateway",
			Confidence:   zombie.Medium,
			Reason:       fmt.Sprintf("%.0f bytes out over %d days (checked %s)", sum, cfg.IdleWindowDays, inv.Now.Format("2006-01-02")),
			CreatedAt:    &created,
			Tags:         n.Tags,
		}
		f.Meta("bytes_out", strconv.FormatFloat(sum, 'f', 0, 64))
		f.Meta("window_days", strconv.Itoa(cfg.IdleWindowDays))
		f.Meta("vpc_id", n.VPCID)

		out = append(out, f)

	}
	return out
}
