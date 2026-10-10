package detect

import (
	"fmt"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(kmsKeyDisabled{})
}

type kmsKeyDisabled struct{}

func (kmsKeyDisabled) Name() string {
	return "kms-key-disabled"
}

func (kmsKeyDisabled) Describe() string {
	return "Disabled customer-managed KMS keys - unusable by definition, still $1/month each"
}

func (kmsKeyDisabled) Needs() []string {
	return []string{
		"kms:ListKeys",
		"kms:DescribeKey",
	}
}

func (kmsKeyDisabled) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, k := range inv.KMSKeys {
		// AWS-managed keys are free
		if k.Manager != "CUSTOMER" {
			continue
		}
		if k.State != "Disabled" {
			continue
		}
		if inv.AgeDays(k.CreatedAt) < cfg.MinAgeDays {
			continue
		}

		what := k.Description
		if what == "" {
			what = "no description"
		}

		created := k.CreatedAt
		f := zombie.Finding{
			ResourceID:   k.ID,
			ResourceType: "kms-key",
			ResourceARN:  k.ARN,
			Confidence:   zombie.High,
			Reason: fmt.Sprintf("Disabled customer-managed key, created %d days ago (%s)",
				inv.AgeDays(k.CreatedAt), what),
			CreatedAt: &created,
		}
		f.Meta("key_state", k.State)
		f.Meta("key_manager", k.Manager)

		out = append(out, f)
	}

	return out
}
