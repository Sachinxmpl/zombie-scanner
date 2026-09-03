package zombie

import "time"

const SchemaVersion = "1"

type ToolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
}

// Complete result of one scan
type Report struct {
	SchemaVersion string         `json:"schema_version"`
	Tool          ToolInfo       `json:"tool"`
	AccountID     string         `json:"account_id"`
	ScannedAt     time.Time      `json:"scanned_at"`
	Regions       []string       `json:"regions"`
	Findings      []Finding      `json:"findings"` // never null
	Errors        []ScanError    `json:"errors"`   // never null
	Summary       Summary        `json:"summary"`
	Filtered      map[string]int `json:"filtered_out,omitempty"`
}

type Summary struct {
	TotalMonthlyUSD float64            `json:"total_monthly_usd"`
	ZombieCount     int                `json:"zombie_count"`
	ByConfidence    map[string]int     `json:"by_confidence"`
	ByDetector      map[string]float64 `json:"by_detector_monthly_usd"`
}

func (r *Report) Normalize() {
	if r.SchemaVersion == "" {
		r.SchemaVersion = SchemaVersion
	}
	if r.Regions == nil {
		r.Regions = []string{}
	}
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	if r.Errors == nil {
		r.Errors = []ScanError{}
	}
	if r.Summary.ByConfidence == nil {
		r.Summary.ByConfidence = map[string]int{}
	}
	if r.Summary.ByDetector == nil {
		r.Summary.ByDetector = map[string]float64{}
	}
}
