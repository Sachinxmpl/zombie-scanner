package detect

import (
	"fmt"
	"strconv"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(efsUnused{})
}

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

		total := fs.StandardGiB + fs.IAGiB + fs.ArchiveGiB
		if total <= 0 {
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
			Reason:       fmt.Sprintf("No mount targets; %d GiB still billing (%s, created %d days ago)", total, name, inv.AgeDays(created)),
			CreatedAt:    &created,
			Tags:         fs.Tags,
		}

		f.Meta("name", name)
		f.Meta("standard_gib", strconv.Itoa(int(fs.StandardGiB)))
		f.Meta("ia_gib", strconv.Itoa(int(fs.IAGiB)))
		f.Meta("archive_gib", strconv.Itoa(int(fs.ArchiveGiB)))

		out = append(out, f)
	}

	return out
}
