package zombie

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Confidence uint8

// zero value -> confidence not set, unknown
const (
	Low Confidence = iota + 1
	Medium
	High
)

func (c Confidence) Rank() int {
	return int(c)
}

func (c Confidence) String() string {
	switch c {
	case High:
		return "HIGH"
	case Medium:
		return "MEDIUM"
	case Low:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

func (c Confidence) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

func (c *Confidence) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("confidence: %w", err)
	}
	parsed, err := ParseConfidence(s)
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

func ParseConfidence(s string) (Confidence, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "HIGH":
		return High, nil
	case "MEDIUM":
		return Medium, nil
	case "LOW":
		return Low, nil
	default:
		return 0, fmt.Errorf("unknown confidence %q (want HIGH, MEDIUM or LOW)", s)
	}
}
