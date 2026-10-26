package test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sachinxmpl/zombie-scanner/cli"
)

// Filling a flag from env or config marks it Changed, so a region set there must not count as typed, it is only the starting region for --all-regions.
func TestAllRegionsOverridesConfigAndEnvRegion(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(cfg, []byte("region: eu-west-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		env       string
		args      []string
		wantClash bool
	}{
		{"both typed", "", []string{"scan", "--region", "eu-west-1", "--all-regions", "--no-config"}, true},
		{"region from config", "", []string{"scan", "--all-regions", "--config", cfg}, false},
		{"region from env", "eu-west-1", []string{"scan", "--all-regions", "--no-config"}, false},
		{"typed region with config", "", []string{"scan", "--region", "us-east-1", "--config", cfg}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv("ZOMBIE_SCANNER_REGION", tc.env)
			}
			root := cli.NewRootCommand("test", "test")
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			// a profile that cannot exist stops the run before any AWS call
			root.SetArgs(append(tc.args, "--profile", "no-such-profile-xyz"))

			err := root.Execute()
			if err == nil {
				t.Fatal("expected an error from the missing profile")
			}
			clash := strings.Contains(err.Error(), "cannot be used together") ||
				strings.Contains(err.Error(), "flags in the group")
			if clash != tc.wantClash {
				t.Errorf("got %q, want clash=%v", err, tc.wantClash)
			}
			if !tc.wantClash && !strings.Contains(err.Error(), "no-such-profile-xyz") {
				t.Errorf("got %q, want it to get as far as loading the profile", err)
			}
		})
	}
}
