package detect

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(rdsIdle{})
}

type rdsIdle struct{}

func (rdsIdle) Name() string {
	return "rds-idle"
}

func (rdsIdle) Describe() string {
	return "Running RDS instances that nothing connected to over the metric window"
}

func (rdsIdle) Needs() []string {
	return []string{
		"rds:DescribeDBInstances",
		"cloudwatch:GetMetricData",
	}
}

func (rdsIdle) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}
	window := time.Duration(cfg.IdleWindowDays) * 24 * time.Hour

	for _, db := range inv.DBInstances {
		// Stopped instances belong to rds-stopped detector.
		if db.Status != "available" {
			continue
		}

		// Aurora bills instance-hours on its own schedule, and Serverless v2 reports db.serverless and bills ACUs.
		// Skip for now
		if strings.HasPrefix(db.Engine, "aurora") {
			continue
		}

		// no history, no verdict
		if inv.Now.Sub(db.CreatedAt) < window {
			continue
		}

		sum, ok := inv.Metrics.Sum(zombie.MetricKey{
			NameSpace:  "AWS/RDS",
			Metric:     "DatabaseConnections",
			ResourceID: db.ID,
		})
		// missing data is unknown -> not idle
		if !ok {
			continue
		}
		if sum >= cfg.RDSIdleConnections {
			continue
		}

		role := "instance"
		if db.ReplicaOf != "" {
			role = "read replica of " + db.ReplicaOf
		}

		created := db.CreatedAt
		f := zombie.Finding{
			ResourceID:   db.ID,
			ResourceType: "rds-instance",
			ResourceARN:  db.ARN,
			Confidence:   zombie.Medium,
			Reason: fmt.Sprintf("Running %s with no connections over %d days (%s, %s, checked %s)",
				role, cfg.IdleWindowDays, db.Class, db.Engine, inv.Now.Format("2006-01-02")),
			CreatedAt: &created,
			Tags:      db.Tags,
		}
		f.Meta("status", db.Status)
		f.Meta("engine", db.Engine)
		f.Meta("instance_class", db.Class)
		f.Meta("storage_gib", strconv.Itoa(int(db.StorageGiB)))
		f.Meta("storage_type", db.StorageType)
		f.Meta("multi_az", strconv.FormatBool(db.MultiAZ))
		f.Meta("connection_minutes", strconv.FormatFloat(sum, 'f', 0, 64))
		f.Meta("window_days", strconv.Itoa(cfg.IdleWindowDays))
		if db.ReplicaOf != "" {
			f.Meta("read_replica_of", db.ReplicaOf)
		}

		out = append(out, f)
	}

	return out
}
