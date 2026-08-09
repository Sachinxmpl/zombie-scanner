package zombie

import "time"

// Identifies one cloudwatch series
type MetricKey struct {
	NameSpace  string
	Metric     string
	ResourceID string
}

// holds CloudWatch data
type MetricSet struct {
	values map[MetricKey]float64
	window time.Duration
}

func NewMetricSet(window time.Duration) MetricSet {
	return MetricSet{
		values: make(map[MetricKey]float64),
		window: window,
	}
}

func (m *MetricSet) Set(k MetricKey, v float64) {
	if m.values == nil {
		m.values = make(map[MetricKey]float64)
	}
	m.values[k] = v
}

// Returns the value summed over the window
// ok == false -> means CloudWatch returned no datapoints - which means unknown (not zero)
func (m MetricSet) Sum(k MetricKey) (float64, bool) {
	v, ok := m.values[k]
	return v, ok
}

func (m MetricSet) Window() time.Duration {
	return m.window
}

// Reports ho wmany series were collected
func (m MetricSet) Len() int {
	return len(m.values)
}
