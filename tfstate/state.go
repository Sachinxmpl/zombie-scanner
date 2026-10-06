package tfstate

import (
	"encoding/json"
	"fmt"
	"io"
)

// Reads terraform state files and reports resources it manages

type stateFile struct {
	Version   int `json:"version"`
	Resources []struct {
		Mode      string `json:"mode"`
		Instances []struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"instances"`
	} `json:"resources"`
}

type Managed struct {
	ids   map[string]bool
	files []string
}

func (m *Managed) Has(s string) bool {
	return s != "" && m.ids[s]
}

func (m *Managed) Len() int {
	return len(m.ids)
}

func (m *Managed) Files() []string {
	return m.files
}

func (m *Managed) read(r io.Reader) error {
	var s stateFile

	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return fmt.Errorf("not a terraform state file: %w", err)
	}
	if s.Version == 0 {
		return fmt.Errorf("not a terraform state file: no version field")
	}
	for _, res := range s.Resources {
		if res.Mode != "managed" {
			continue
		}
		for _, inst := range res.Instances {
			for _, key := range []string{"id", "arn"} {
				raw, ok := inst.Attributes[key]
				if !ok {
					continue
				}
				var v string
				if json.Unmarshal(raw, &v) != nil && v != "" {
					m.ids[v] = true
				}
			}
		}
	}
	return nil
}
