package detect

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(rdsStopped{})
}

type rdsStopped struct{}

func (rdsStopped) Name() string {
	return "rds-stopped"
}

func (rdsStopped) Describe() string {
	return "Stopped RDS instances whose storage keeps billing, and that AWS restarts after 7 days"
}

func (rdsStopped) Needs() []string {
	return []string{
		"rds:DescribeDBInstances",
	}
}

func (rdsStopped) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, db := range inv.DBInstances {
		if db.Status != "stopped" {
			continue
		}

		// Aurora storage bills on the cluster by consumption, so a member's
		// AllocatedStorage reads 1 and says nothing about the bill. It still
		// bills while stopped - we just cannot size it from here.
		if strings.HasPrefix(db.Engine, "aurora") {
			continue
		}
		if db.StorageGiB <= 0 {
			continue
		}
		if inv.AgeDays(db.CreatedAt) < cfg.MinAgeDays {
			continue
		}

		conf := zombie.Medium
		reason := fmt.Sprintf("Stopped; %d GiB of %s storage still billing (%s)", db.StorageGiB, db.StorageType, db.Engine)

		if db.AutoRestartAt != nil {
			conf = zombie.High
			reason = fmt.Sprintf("Stopped; %d GiB of %s storage still billing, and AWS restarts it on %s (%s)",
				db.StorageGiB, db.StorageType,
				db.AutoRestartAt.Format("2006-01-02"), db.Engine)
		}

		created := db.CreatedAt
		f := zombie.Finding{
			ResourceID:   db.ID,
			ResourceType: "rds-instance",
			ResourceARN:  db.ARN,
			Confidence:   conf,
			Reason:       reason,
			CreatedAt:    &created,
			Tags:         db.Tags,
		}
		f.Meta("engine", db.Engine)
		f.Meta("instance_class", db.Class)
		f.Meta("storage_gib", strconv.Itoa(int(db.StorageGiB)))
		f.Meta("storage_type", db.StorageType)
		f.Meta("multi_az", strconv.FormatBool(db.MultiAZ))
		if db.AutoRestartAt != nil {
			f.Meta("auto_restart_at", db.AutoRestartAt.Format(time.RFC3339))
		}

		out = append(out, f)

	}

	return out
}
