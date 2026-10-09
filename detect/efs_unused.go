package detect

import (
	"fmt"
	"strconv"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(efsUnused{})
}

// what a brand new, empty EFS file system reports
const efsEmptyBytes = 6 << 10

type efsUnused struct{}

func (efsUnused) Name() string {
	return "efs-unused"
}

func (efsUnused) Describe() string {
	return "EFS File Systems with no mount targets - unreachable, still billing per GiB"
}

func (efsUnused) Needs() []string {
	return []string{
		"elasticfilesystem:DescribeFileSystems",
	}
}

func (efsUnused) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, fs := range inv.FileSystems {
		if fs.State != "available" {
			continue
		}
		// only report file systems with no mount targets, as these are unreachable and still billing per GiB
		if fs.MountTargets > 0 {
			continue
		}

		// an empty file system still reports its metadata, so at or under
		// that floor nothing is stored and nothing worth reporting is billed
		total := fs.StandardBytes + fs.IABytes + fs.ArchiveBytes
		if total <= efsEmptyBytes {
			continue
		}
		if inv.AgeDays(fs.CreatedAt) < cfg.MinAgeDays {
			continue
		}

		name := fs.Name
		if name == "" {
			name = fs.ID
		}

		created := fs.CreatedAt
		f := zombie.Finding{
			ResourceID:   fs.ID,
			ResourceType: "efs-filesystem",
			ResourceARN:  fs.ARN,
			Confidence:   zombie.High,
			Reason:       fmt.Sprintf("No mount targets; %s still billing (%s, created %d days ago)", humanBytes(total), name, inv.AgeDays(created)),
			CreatedAt:    &created,
			Tags:         fs.Tags,
		}

		f.Meta("name", name)
		f.Meta("standard_bytes", strconv.FormatInt(fs.StandardBytes, 10))
		f.Meta("ia_bytes", strconv.FormatInt(fs.IABytes, 10))
		f.Meta("archive_bytes", strconv.FormatInt(fs.ArchiveBytes, 10))

		out = append(out, f)
	}

	return out
}

func humanBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	default:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	}
}
