package zombie

import "time"

// Represents one AWS resource that the scanner believes may be a zombie

type Finding struct {
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	ResourceARN  string `json:"resource_arn,omitempty"`
	Region       string `json:"region"`
	AccountID    string `json:"account_id"`
	Detector     string `json:"detector"`

	Confidence Confidence `json:"confidence"`
	Reason     string     `json:"reason"`

	MonthlyCost float64 `json:"monthly_cost_usd"`
	CostBasis   string  `json:"cost_basis,omitempty"`

	CreatedAt *time.Time        `json:"created_at,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`

	// info shared between different scanner stages
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Sets a metadata key
func (f *Finding) Meta(key, value string) {
	if f.Metadata == nil {
		f.Metadata = make(map[string]string, 4)
	}
	f.Metadata[key] = value
}
