package test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// AutomaticRestartTime is the only thing separating HIGH from MEDIUM here, and
// Aurora members must never be priced from AllocatedStorage.
func TestRDSStopped(t *testing.T) {
	restart := now.AddDate(0, 0, 3)

	inv := zombie.Inventory{Now: now, DBInstances: []zombie.DBInstance{
		{ID: "db-with-deadline", Status: "stopped", Engine: "postgres",
			StorageType: "gp3", StorageGiB: 100, CreatedAt: daysAgo(200),
			AutoRestartAt: &restart},
		{ID: "db-no-deadline", Status: "stopped", Engine: "mysql",
			StorageType: "gp2", StorageGiB: 20, CreatedAt: daysAgo(200)},
		{ID: "db-running", Status: "available", Engine: "postgres",
			StorageType: "gp3", StorageGiB: 100, CreatedAt: daysAgo(200)},
		{ID: "db-aurora", Status: "stopped", Engine: "aurora-postgresql",
			StorageType: "aurora", StorageGiB: 1, CreatedAt: daysAgo(200)},
	}}

	got := detect.Run(inv, detect.Defaults(), []string{"rds-stopped"}, nil)

	want := []string{"db-with-deadline", "db-no-deadline"}
	if !slices.Equal(ids(got), want) {
		t.Fatalf("got %v, want %v", ids(got), want)
	}
	if got[0].Confidence != zombie.High {
		t.Errorf("with AutoRestartAt: got %v, want HIGH", got[0].Confidence)
	}
	if got[1].Confidence != zombie.Medium {
		t.Errorf("without AutoRestartAt: got %v, want MEDIUM", got[1].Confidence)
	}
	if !strings.Contains(got[0].Reason, restart.Format("2006-01-02")) {
		t.Errorf("reason omits the restart date: %q", got[0].Reason)
	}
}

func TestRDSMultiAZDoublesStorageCost(t *testing.T) {
	single := zombie.Finding{ResourceType: "rds-instance"}
	single.Meta("storage_gib", "100")
	single.Meta("storage_type", "gp3")
	single.Meta("multi_az", "false")

	multi := zombie.Finding{ResourceType: "rds-instance"}
	multi.Meta("storage_gib", "100")
	multi.Meta("storage_type", "gp3")
	multi.Meta("multi_az", "true")

	out := price.Apply([]zombie.Finding{single, multi}, "us-east-1")

	if out[0].MonthlyCost <= 0 {
		t.Fatalf("single-AZ priced at %v", out[0].MonthlyCost)
	}
	if out[1].MonthlyCost != out[0].MonthlyCost*2 {
		t.Errorf("multi-AZ %.2f, want 2x single-AZ %.2f", out[1].MonthlyCost, out[0].MonthlyCost)
	}
}
