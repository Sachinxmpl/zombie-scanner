package tfstate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func Load(paths []string) (*Managed, error) {
	m := &Managed{ids: map[string]bool{}}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("--tf-state %s: %w", p, err)
		}
		if info.IsDir() {
			err = m.loadDir(p)
		} else {
			err = m.loadFile(p)
		}
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Managed) loadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("--tf-state %s: %w", dir, err)
	}

	found := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tfstate") {
			continue
		}
		if err := m.loadFile(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
		found++
	}
	if found == 0 {
		return fmt.Errorf("--tf-state %s: no .tfstate files found", dir)
	}
	return nil
}

func (m *Managed) loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("--tf-state %s: %w", path, err)
	}
	defer f.Close()

	if err := m.read(f); err != nil {
		return fmt.Errorf("--tf-state %s: %w", path, err)
	}
	m.files = append(m.files, path)
	return nil
}
