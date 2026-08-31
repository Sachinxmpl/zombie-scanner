package render

import (
	"cmp"
	"slices"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// Sort orders findings: cost desc, confidence desc, region asc, id asc
func Sort(fs []zombie.Finding) []zombie.Finding {
	slices.SortFunc(fs, func(a, b zombie.Finding) int {
		// b , a  : descending
		if c := cmp.Compare(b.MonthlyCost, a.MonthlyCost); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Confidence.Rank(), a.Confidence.Rank()); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Region, b.Region); c != 0 {
			return c
		}
		return cmp.Compare(a.ResourceID, b.ResourceID)
	})

	return fs
}
