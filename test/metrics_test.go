package test

import (
	"testing"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func natKey(id string) zombie.MetricKey {
	return zombie.MetricKey{
		NameSpace:  "AWS/NATGateway",
		Metric:     "BytesOutToDestination",
		ResourceID: id,
	}
}

// A real zero and a missing series must not look the same. Reading a missing map key yields 0, and 0 means "idle"
// so this distinction is the only thing between a permissions gap and a confident false positive.
func TestMetricSetDistinguishesZeroFromMissing(t *testing.T) {
	m := zombie.NewMetricSet(14 * 24 * time.Hour)
	m.Set(natKey("nat-present"), 0)

	if v, ok := m.Sum(natKey("nat-present")); !ok || v != 0 {
		t.Errorf("present key: got (%v, %v), want (0, true)", v, ok)
	}
	if v, ok := m.Sum(natKey("nat-absent")); ok {
		t.Errorf("absent key: got (%v, true), want ok == false", v)
	}
}

// Same resource, opposite verdicts, decided only by whether CloudWatch had data.
func TestNATIsFlaggedOnZeroButNotOnMissingData(t *testing.T) {
	nat := zombie.NATGateway{ID: "nat-1", State: "available", CreatedAt: daysAgo(100)}
	window := 14 * 24 * time.Hour

	measured := zombie.NewMetricSet(window)
	measured.Set(natKey("nat-1"), 0)

	for _, tc := range []struct {
		name    string
		metrics zombie.MetricSet
		want    int
	}{
		{"measured zero is evidence", measured, 1},
		{"no datapoints is not evidence", zombie.NewMetricSet(window), 0},
		{"cloudwatch never ran", zombie.MetricSet{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := zombie.Inventory{
				Now:         now,
				NATGateways: []zombie.NATGateway{nat},
				Metrics:     tc.metrics,
			}
			got := detect.Run(inv, detect.Defaults(), []string{"nat-idle"}, nil)
			if len(got) != tc.want {
				t.Errorf("got %d findings, want %d", len(got), tc.want)
			}
		})
	}
}
