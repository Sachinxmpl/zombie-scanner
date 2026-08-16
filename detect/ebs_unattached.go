package detect

import (
	"fmt"
	"strconv"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(ebsUnattached{})
}

type ebsUnattached struct{}

func (ebsUnattached) Name() string {
	return "ebs-unattached"
}

func (ebsUnattached) Describe() string {
	return `EBS volumes in state "available" - attached to nothing, billing per GiB-month`
}

func (ebsUnattached) Needs() []string {
	return []string{
		"ec2:DescribeVolumes",
	}
}

func (ebsUnattached) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, v := range inv.Volumes {
		if v.State != "available" {
			continue
		}

		age := inv.AgeDays(v.CreatedAt)

		if age < cfg.MinAgeDays {
			continue
		}

		created := v.CreatedAt
		f := zombie.Finding{
			ResourceID:   v.ID,
			ResourceType: "ebs-volume",
			Confidence:   zombie.High,

			// Aws doesn't report when volume was detached, only when it was created so created ....
			Reason: fmt.Sprintf("Unattached; created %d days ago (%d GiB %s)",
				age, v.SizeGiB, v.VolumeType),

			CreatedAt: &created,
			Tags:      v.Tags,
		}

		f.Meta("size_gib", strconv.Itoa(int(v.SizeGiB)))
		f.Meta("volume_type", v.VolumeType)

		out = append(out, f)

	}

	return out
}
