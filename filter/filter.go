package filter

import "github.com/Sachinxmpl/zombie-scanner/zombie"

// Filters findings
// Runs right after pricing

type Filter interface {
	Name() string
	Keep(f zombie.Finding) bool
}

// Returns the kept findings, and how many each filter dropped
func Apply(fs []zombie.Finding, filters []Filter) ([]zombie.Finding, map[string]int) {
	if len(filters) == 0 {
		return fs, nil
	}

	kept := make([]zombie.Finding, 0, len(fs))
	dropped := make(map[string]int)

	for _, f := range fs {
		keep := true
		for _, flt := range filters {
			if !flt.Keep(f) {
				dropped[flt.Name()]++
				keep = false
				break
			}
		}
		if keep {
			kept = append(kept, f)
		}
	}
	return kept, dropped
}

type MinCost struct {
	USD float64
}

func (MinCost) Name() string {
	return "--min-cost"
}
func (m MinCost) Keep(f zombie.Finding) bool {
	return f.MonthlyCost >= m.USD
}

type MinConfidence struct {
	Level zombie.Confidence
}

func (MinConfidence) Name() string {
	return "--confidence"
}
func (m MinConfidence) Keep(f zombie.Finding) bool {
	return f.Confidence.Rank() >= m.Level.Rank()
}
