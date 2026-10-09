package detect

import (
	"fmt"
	"strconv"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(rdsSnapshotAged{})
}

type rdsSnapshotAged struct{}

func (rdsSnapshotAged) Name() string {
	return "rds-snapshot-aged"
}

func (rdsSnapshotAged) Describe() string {
	return "Manual RDS snapshots older than the threshold, kept forever unless deleted by hand"
}

func (rdsSnapshotAged) Needs() []string {
	return []string{
		"rds:DescribeDBSnapshots",
	}
}
func (rdsSnapshotAged) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, snap := range inv.DBSnapshots {
		if snap.Type != "manual" || snap.Status != "available" {
			continue
		}
		age := inv.AgeDays(snap.CreatedAt)
		if age < cfg.SnapshotAgeDays {
			continue
		}

		size, exact := snap.ActualBytes, true
		if size <= 0 {
			size, exact = int64(snap.StorageGiB)<<30, false
		}
		if size <= 0 {
			continue
		}

		created := snap.CreatedAt
		f := zombie.Finding{
			ResourceID:   snap.ID,
			ResourceType: "rds-snapshot",
			ResourceARN:  snap.ARN,
			Confidence:   zombie.Low,
			Reason: fmt.Sprintf("Manual snapshot of %s, %d days old (%s, %s)",
				snap.DBInstance, age, humanBytes(size), snap.Engine),
			CreatedAt: &created,
			Tags:      snap.Tags,
		}
		f.Meta("db_instance", snap.DBInstance)
		f.Meta("engine", snap.Engine)
		f.Meta("size_bytes", strconv.FormatInt(size, 10))
		if !exact {
			f.Meta("size_is_instance_allocation", "true")
		}
		out = append(out, f)
	}

	return out
}
