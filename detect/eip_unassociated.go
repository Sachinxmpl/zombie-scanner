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
	return "Elastic IPs that reach nothing: unattached, or on a detached network interface - AWS bills every public IPv4 address"
}

func (eipUnassociated) Needs() []string {
	return []string{
		"ec2:DescribeAddresses",
		"ec2:DescribeNetworkInterfaces",
	}
}

func (eipUnassociated) Detect(inv zombie.Inventory, cfg Config) []zombie.Finding {
	out := []zombie.Finding{}

	for _, a := range inv.Addresses {
		var conf zombie.Confidence
		var reason string
		switch {
		// AWS sets AssociationId whenever an address is attached to an instance or a network interface
		case a.AssociationID == "":
			conf = zombie.High
			reason = fmt.Sprintf("Public IP %s is allocated but associated with nothing", a.PublicIP)
		// a spare interface kept for failover looks the same. -> medium confidence
		case a.InterfaceDetached:
			conf = zombie.Medium
			reason = fmt.Sprintf("Public IP %s is on network interface %s, which is attached to nothing",
				a.PublicIP, a.NetworkInterfaceID)
		default:
			continue
		}

		id := a.AllocationID
		if id == "" {
			id = a.PublicIP // pre-VPC addresses have no allocation id
		}

		f := zombie.Finding{
			ResourceID:   id,
			ResourceType: "elastic-ip",
			Confidence:   conf,
			Reason:       reason,
			Tags:         a.Tags,
		}
		f.Meta("public_ip", a.PublicIP)
		if a.NetworkInterfaceID != "" {
			f.Meta("network_interface", a.NetworkInterfaceID)
		}

		out = append(out, f)
	}

	return out
}
