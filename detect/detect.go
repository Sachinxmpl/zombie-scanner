package detect

import "github.com/Sachinxmpl/zombie-scanner/zombie"

type Config struct {
	// how old resource must be before any detector will judge it
	MinAgeDays int

	SnapshotAgeDays int
	StoppedDays     int
	IdelWindowDays  int

	NATIdleBytes    float64
	ELBIdleRequests float64
}

type Detector interface {
	Name() string
	Describe() string
	Needs() []string
	Detect(inv zombie.Inventory, cfg Config) []zombie.Finding
}

func Defaults() Config {
	return Config{
		MinAgeDays:      1,
		SnapshotAgeDays: 90,
		StoppedDays:     30,
		IdelWindowDays:  14,
		NATIdleBytes:    1 << 20, // 1MiB
		ELBIdleRequests: 1,
	}
}

var registry []Detector

func Register(d Detector) {
	registry = append(registry, d)
}

// Returns copy of registry
func All() []Detector {
	out := make([]Detector, len(registry))
	copy(out, registry)
	return out
}

// Applies every selected detector to one inventory
// Adds Region, AccountID to each finding
// only -> if non-eempty -> run just these detectors
// skip -> never run these, skip
func Run(inv zombie.Inventory, cfg Config, only, skip []string) []zombie.Finding {
	findings := []zombie.Finding{}

	onlySet := toSet(only)
	skipSet := toSet(skip)

	for _, d := range registry {
		name := d.Name()
		if len(onlySet) > 0 && !onlySet[name] {
			continue
		}
		if skipSet[name] {
			continue
		}

		for _, f := range d.Detect(inv, cfg) {
			f.Detector = name
			f.Region = inv.Region
			f.AccountID = inv.AccountID
			findings = append(findings, f)
		}
	}

	return findings
}

func toSet(xs []string) map[string]bool {
	if len(xs) == 0 {
		return nil
	}
	m := make(map[string]bool, len(xs))

	for _, x := range xs {
		m[x] = true
	}

	return m
}
