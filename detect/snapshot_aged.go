package detect

import (
	"fmt"
	"strconv"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(snapshotAged{})
}

type snapshotAged struct{}

func (snapshotAged) Name() string {
	return "snapshot-aged"
}

func (snapshotAged) Describe() string {
	return "EBS snapshots older than the threshold, and not used by any AMI, are likely abandoned"
}

func (snapshotAged) Needs() []string {
	return []string{
		"ec2:DescribeSnapshots",
		"ec2:DescribeImages",
	}
}
func (snapshotAged) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	// deleting a snapshot behind a registered AMI breaks the AMI
	// So if corss-reference could not be build, flag nothing at all
	if inv.Missing("ec2:DescribeImages") {
		return out
	}

	inUse := make(map[string]bool)
	for _, i := range inv.Images {
		for _, id := range i.SnapshotIDs {
			inUse[id] = true
		}
	}

	for _, s := range inv.Snapshots {
		if inUse[s.ID] {
			continue
		}

		age := inv.AgeDays(s.StartedAt)
		if age < cfg.SnapshotAgeDays {
			continue
		}

		started := s.StartedAt
		f := zombie.Finding{
			ResourceID:   s.ID,
			ResourceType: "ebs-snapshot",
			Confidence:   zombie.Low,
			Reason:       fmt.Sprintf("Created %d days ago (%d GiB), not referenced by any AMI", age, s.SizeGiB),
			CreatedAt:    &started,
			Tags:         s.Tags,
		}

		f.Meta("size_gib", strconv.Itoa(int(s.SizeGiB)))
		f.Meta("source_volume", s.VolumeID)

		out = append(out, f)
	}

	return out
}
