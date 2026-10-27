package filter

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

type IgnoreRule struct {
	ID       string `yaml:"id"`
	Detector string `yaml:"detector"`
	Region   string `yaml:"region"`
	Reason   string `yaml:"reason"`
}

func (r IgnoreRule) String() string {
	parts := make([]string, 0, 3)
	if r.ID != "" {
		parts = append(parts, "id="+r.ID)
	}
	if r.Detector != "" {
		parts = append(parts, "detector="+r.Detector)
	}
	if r.Region != "" {
		parts = append(parts, "region="+r.Region)
	}
	return strings.Join(parts, " ")
}

func (r IgnoreRule) matches(f zombie.Finding) bool {
	if r.ID != "" && r.ID != f.ResourceID {
		return false
	}
	if r.Detector != "" && r.Detector != f.Detector {
		return false
	}
	if r.Region != "" && r.Region != f.Region {
		return false
	}
	return true
}

// Reject finding matched by any rule
type IgnoreRules struct {
	rules []IgnoreRule
	hits  []atomic.Int64
}

func NewIgnoreRules(rules []IgnoreRule) *IgnoreRules {
	return &IgnoreRules{
		rules: rules,
		hits:  make([]atomic.Int64, len(rules)),
	}
}

func (*IgnoreRules) Name() string {
	return "ignore rules"
}

func (r *IgnoreRules) Keep(f zombie.Finding) bool {
	for i := range r.rules {
		if r.rules[i].matches(f) {
			r.hits[i].Add(1)
			return false
		}
	}
	return true
}

// rules that matched nothing on this run
func (r *IgnoreRules) Unmatched() []IgnoreRule {
	out := []IgnoreRule{}
	for i := range r.rules {
		if r.hits[i].Load() == 0 {
			out = append(out, r.rules[i])
		}
	}
	return out
}

func ValidateIgnoreRules(rules []IgnoreRule, knownDetector func(string) bool) error {
	for i, r := range rules {
		if r.ID == "" && r.Detector == "" && r.Region == "" {
			return fmt.Errorf("ignore rule %d matches every finding: set at least one of id, detector, region", i+1)
		}
		if strings.TrimSpace(r.Reason) == "" {
			return fmt.Errorf("ignore rule %d (%s) has no reason: every ignore rule must say why", i+1, r)
		}
		if r.Detector != "" && !knownDetector(r.Detector) {
			return fmt.Errorf("ignore rule %d: unknown detector %q", i+1, r.Detector)
		}
	}
	return nil
}
