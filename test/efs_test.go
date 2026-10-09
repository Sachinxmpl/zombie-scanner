package test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

const gibBytes = 1 << 30

func TestEFSUnused(t *testing.T) {
	inv := zombie.Inventory{Now: now, FileSystems: []zombie.FileSystem{
		{ID: "fs-orphan", Name: "old-uploads", State: "available",
			StandardBytes: 200 * gibBytes, CreatedAt: daysAgo(300)},
		// under 1 GiB still bills; whole-GiB sizes once rounded this to 0
		{ID: "fs-small", State: "available",
			StandardBytes: 900 << 20, CreatedAt: daysAgo(300)},
		{ID: "fs-mounted", State: "available", MountTargets: 3,
			StandardBytes: 200 * gibBytes, CreatedAt: daysAgo(300)},
		{ID: "fs-deleting", State: "deleting",
			StandardBytes: 200 * gibBytes, CreatedAt: daysAgo(300)},
		// a brand new, empty file system reports 6 KiB of metadata
		{ID: "fs-empty", State: "available",
			StandardBytes: 6 << 10, CreatedAt: daysAgo(300)},
	}}

	got := detect.Run(inv, detect.Defaults(), []string{"efs-unused"}, nil)

	if want := []string{"fs-orphan", "fs-small"}; !slices.Equal(ids(got), want) {
		t.Fatalf("got %v, want %v", ids(got), want)
	}
	if got[0].Confidence != zombie.High {
		t.Errorf("confidence = %v, want HIGH", got[0].Confidence)
	}
	if !strings.Contains(got[0].Reason, "old-uploads") {
		t.Errorf("reason does not name the file system: %q", got[0].Reason)
	}
	if !strings.Contains(got[1].Reason, "900.0 MiB") {
		t.Errorf("reason hides the real size: %q", got[1].Reason)
	}
}

func TestEFSPricesExactSizePerTier(t *testing.T) {
	mk := func(std, ia string) zombie.Finding {
		f := zombie.Finding{ResourceType: "efs-filesystem"}
		f.Meta("standard_bytes", std)
		f.Meta("ia_bytes", ia)
		f.Meta("archive_bytes", "0")
		return f
	}

	out := price.Apply([]zombie.Finding{
		mk("107374182400", "0"),            // 100 GiB standard
		mk("0", "107374182400"),            // 100 GiB IA
		mk("536870912", "0"),               // 0.5 GiB standard
		mk("107374182400", "107374182400"), // both
	}, "us-east-1")
	std, ia, half, both := out[0], out[1], out[2], out[3]

	if ia.MonthlyCost >= std.MonthlyCost/10 {
		t.Errorf("IA %.2f vs standard %.2f - IA is not priced at its own rate",
			ia.MonthlyCost, std.MonthlyCost)
	}
	if math.Abs(half.MonthlyCost-std.MonthlyCost/200) > 0.0001 {
		t.Errorf("0.5 GiB = $%.4f, want $%.4f - fractions of a GiB are lost",
			half.MonthlyCost, std.MonthlyCost/200)
	}
	if math.Abs(both.MonthlyCost-(std.MonthlyCost+ia.MonthlyCost)) > 0.001 {
		t.Errorf("tiers do not sum: %.4f", both.MonthlyCost)
	}
	if !strings.Contains(both.CostBasis, "standard") || !strings.Contains(both.CostBasis, "ia") {
		t.Errorf("cost basis hides the tiers: %q", both.CostBasis)
	}
}
