package detect

import (
	"fmt"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func init() {
	Register(eipUnassociated{})
}

type eipUnassociated struct{}

func (eipUnassociated) Name() string {
	return "eip-unassociated"
}

func (eipUnassociated) Describe() string {
	return "Elastic IPs attached to nothing - AWS bills every public IPv4 address it allocates"
}

func (eipUnassociated) Needs() []string {
	return []string{
		"ec2:DescribeAddresses",
	}
}

func (eipUnassociated) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, a := range inv.Addresses {
		// AWS sets AssociationId whenever an address is attached to an instance or a network interface.
		if a.AssociationID != "" {
			continue
		}

		// DescribeAddresses carries no allocation timestamp, so cfg.MinAgeDays cannot be applied here.
		id := a.AllocationID
		if id == "" {
			id = a.PublicIP // pre-VPC addresses have no allocation id
		}

		f := zombie.Finding{
			ResourceID:   id,
			ResourceType: "elastic-ip",
			Confidence:   zombie.High,
			Reason: fmt.Sprintf("Public IP %s is allocated but associated with nothing",
				a.PublicIP),
			Tags: a.Tags,
		}
		f.Meta("public_ip", a.PublicIP)

		out = append(out, f)
	}

	return out
}
