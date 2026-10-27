package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sachinxmpl/zombie-scanner/filter"
	"github.com/Sachinxmpl/zombie-scanner/tfstate"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

const fixture = `{
  "version": 4,
  "resources": [
    {"mode": "managed", "type": "aws_db_instance", "name": "main",
     "instances": [{"attributes": {
        "id": "database-1",
        "arn": "arn:aws:rds:ap-south-1:123456789012:db:database-1"}}]},
    {"mode": "managed", "type": "aws_ebs_volume", "name": "data",
     "instances": [{"attributes": {"id": "vol-0managed"}}]},
    {"mode": "data", "type": "aws_vpc", "name": "default",
     "instances": [{"attributes": {
        "id": "vpc-0datablock",
        "arn": "arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-0datablock"}}]}
  ]
}`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStateCollectsManagedIDsAndARNsOnly(t *testing.T) {
	p := writeFile(t, t.TempDir(), "terraform.tfstate", fixture)

	m, err := tfstate.Load([]string{p})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"database-1",
		"arn:aws:rds:ap-south-1:123456789012:db:database-1",
		"vol-0managed",
	} {
		if !m.Has(want) {
			t.Errorf("expected %q to be managed", want)
		}
	}

	if m.Has("vpc-0datablock") {
		t.Error("data block id must not count as managed")
	}
	if m.Has("arn:aws:ec2:ap-south-1:123456789012:vpc/vpc-0datablock") {
		t.Error("data block arn must not count as managed")
	}

	if m.Has("") {
		t.Error(`Has("") must be false`)
	}
	if m.Len() != 3 {
		t.Errorf("Len = %d, want 3", m.Len())
	}
}

func TestStateRejectsWhatIsNotState(t *testing.T) {
	dir := t.TempDir()
	plainJSON := writeFile(t, dir, "plain.tfstate", `{"hello":"world"}`)
	notJSON := writeFile(t, dir, "main.tf", `resource "aws_instance" "x" {}`)
	emptyDir := t.TempDir()

	cases := map[string][]string{
		"valid JSON, no version":  {plainJSON},
		"not JSON at all":         {notJSON},
		"missing path":            {filepath.Join(dir, "nope.tfstate")},
		"directory with no state": {emptyDir},
	}
	for name, paths := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := tfstate.Load(paths); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// a fake set, to show filter needs no file and no tfstate import
type fakeSet map[string]bool

func (f fakeSet) Has(s string) bool { return s != "" && f[s] }

func TestUnmanagedDropsWhatTerraformOwns(t *testing.T) {
	u := &filter.Unmanaged{Managed: fakeSet{
		"vol-byid": true,
		"arn:aws:elasticloadbalancing:ap-south-1:1:loadbalancer/app/x/1": true,
	}}

	byID := zombie.Finding{ResourceID: "vol-byid"}
	byARN := zombie.Finding{
		ResourceID:  "x",
		ResourceARN: "arn:aws:elasticloadbalancing:ap-south-1:1:loadbalancer/app/x/1",
	}
	unowned := zombie.Finding{ResourceID: "vol-unowned"}
	noARN := zombie.Finding{ResourceID: "vol-other"} // empty ARN must not match

	if u.Keep(byID) {
		t.Error("match by id should be dropped")
	}
	if u.Keep(byARN) {
		t.Error("match by ARN should be dropped")
	}
	if !u.Keep(unowned) {
		t.Error("unmanaged finding should be kept")
	}
	if !u.Keep(noARN) {
		t.Error("finding with empty ARN must not be suppressed")
	}
	if u.Dropped() != 2 {
		t.Errorf("Dropped = %d, want 2", u.Dropped())
	}
	if u.Name() != "--tf-state" {
		t.Errorf("Name = %q", u.Name())
	}
}
