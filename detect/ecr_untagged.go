package detect

import (
	"fmt"
	"strconv"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(ecrUntagged{})
}

type ecrUntagged struct{}

func (ecrUntagged) Name() string {
	return "ecr-untagged"
}

func (ecrUntagged) Describe() string {
	return "Untagged container images past the age threshold, reachable only by digest"
}

func (ecrUntagged) Needs() []string {
	return []string{
		"ecr:DescribeRepositories",
		"ecr:DescribeImages",
	}
}

func (ecrUntagged) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, img := range inv.ECRImages {
		if len(img.Tags) > 0 || img.SizeBytes <= 0 {
			continue
		}

		age := inv.AgeDays(img.PushedAt)
		if age < cfg.SnapshotAgeDays {
			continue
		}

		conf := zombie.Low
		pulled := "no pull on record"
		if img.LastPulled != nil {
			days := inv.AgeDays(*img.LastPulled)
			pulled = fmt.Sprintf("last pulled %d days ago", days)
			if days >= cfg.SnapshotAgeDays {
				conf = zombie.Medium
			}
		}

		pushed := img.PushedAt
		f := zombie.Finding{
			ResourceID:   short(img.Digest),
			ResourceType: "ecr-image",
			ResourceARN:  img.RepoARN,
			Confidence:   conf,
			Reason: fmt.Sprintf("Untagged in %s, pushed %d days ago (%s, %s)",
				img.Repository, age, humanBytes(img.SizeBytes), pulled),
			CreatedAt: &pushed,
		}
		f.Meta("repository", img.Repository)
		f.Meta("digest", img.Digest)
		f.Meta("size_bytes", strconv.FormatInt(img.SizeBytes, 10))

		out = append(out, f)
	}

	return out
}

// the full digest is 71 characters; "sha256:" plus 12 hex is enough to tell images apart
func short(digest string) string {
	const keep = 19
	if len(digest) <= keep {
		return digest
	}
	return digest[:keep]
}
