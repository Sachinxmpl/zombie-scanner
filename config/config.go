package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Sachinxmpl/zombie-scanner/filter"
	"go.yaml.in/yaml/v3"
)

// Loads .zombie-scanner.yaml
// Precedence: flag > environment > config file > builtin default

const FileName = ".zombie-scanner.yaml"

type File struct {
	Region     *string `yaml:"region"`
	Profile    *string `yaml:"profile"`
	Output     *string `yaml:"output"`
	MinCost    *string `yaml:"min-cost"`
	Confidence *string `yaml:"confidence"`

	SnapshotAgeDays *int `yaml:"snapshot-age-days"`
	StoppedDays     *int `yaml:"stopped-days"`
	IdleWindowDays  *int `yaml:"idle-window-days"`

	KeepTag *string `yaml:"keep-tag"`

	Ignore []filter.IgnoreRule `yaml:"ignore"`

	Path string `yaml:"-"`
}

func (f *File) FlagValues() map[string]string {
	out := map[string]string{}
	set := func(name string, v *string) {
		if v != nil {
			out[name] = *v
		}
	}
	setInt := func(name string, v *int) {
		if v != nil {
			out[name] = strconv.Itoa(*v)
		}
	}

	set("region", f.Region)
	set("profile", f.Profile)
	set("output", f.Output)
	set("min-cost", f.MinCost)
	set("confidence", f.Confidence)
	set("keep-tag", f.KeepTag)
	setInt("snapshot-age-days", f.SnapshotAgeDays)
	setInt("stopped-days", f.StoppedDays)
	setInt("idle-window-days", f.IdleWindowDays)

	return out
}

func Load(path string) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	defer fh.Close()

	f, err := decode(fh)
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	f.Path = path
	return f, nil
}

// Looks for .zombie-scanner.yaml in the working directory, then the home directory
func Discover() (*File, error) {
	candidates := []string{FileName}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, FileName))
	}

	for _, path := range candidates {
		switch _, err := os.Stat(path); {
		case err == nil:
			return Load(path)
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
	}
	return nil, nil
}

func decode(r io.Reader) (*File, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)

	var f File
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return &File{}, nil // an empty file is a valid config
		}
		return nil, err
	}
	return &f, nil
}
