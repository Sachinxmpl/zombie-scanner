package test

import (
	"context"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/Sachinxmpl/zombie-scanner/awsapi/fake"
	"github.com/Sachinxmpl/zombie-scanner/collect"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func TestRDSSnapshotAged(t *testing.T) {
	inv := zombie.Inventory{Now: now, DBSnapshots: []zombie.DBSnapshot{
		{ID: "snap-old", DBInstance: "prod", Type: "manual", Status: "available",
			Engine: "postgres", StorageGiB: 100, ActualBytes: 40 << 30, CreatedAt: daysAgo(400)},
		{ID: "snap-recent", DBInstance: "prod", Type: "manual", Status: "available",
			StorageGiB: 100, ActualBytes: 40 << 30, CreatedAt: daysAgo(10)},
		{ID: "snap-automated", DBInstance: "prod", Type: "automated", Status: "available",
			StorageGiB: 100, ActualBytes: 40 << 30, CreatedAt: daysAgo(400)},
		{ID: "snap-creating", DBInstance: "prod", Type: "manual", Status: "creating",
			StorageGiB: 100, ActualBytes: 40 << 30, CreatedAt: daysAgo(400)},
	}}

	got := detect.Run(inv, detect.Defaults(), []string{"rds-snapshot-aged"}, nil)

	if !slices.Equal(ids(got), []string{"snap-old"}) {
		t.Fatalf("got %v, want [snap-old]", ids(got))
	}
	if got[0].Confidence != zombie.Low {
		t.Errorf("confidence = %v, want LOW - age is a guess", got[0].Confidence)
	}
	if got[0].Metadata["size_bytes"] != "42949672960" {
		t.Errorf("size = %s, want the snapshot's 40 GiB, not the instance's 100",
			got[0].Metadata["size_bytes"])
	}
}

// Whole-GiB sizes turned a 500 MiB snapshot into 0, which then fell back to
// the instance allocation and priced it 40x too high.
func TestRDSSnapshotUnderOneGiBKeepsItsRealSize(t *testing.T) {
	inv := zombie.Inventory{Now: now, DBSnapshots: []zombie.DBSnapshot{
		{ID: "snap-small", DBInstance: "dev", Type: "manual", Status: "available",
			StorageGiB: 20, ActualBytes: 500 << 20, CreatedAt: daysAgo(400)},
	}}

	got := price.Apply(detect.Run(inv, detect.Defaults(), []string{"rds-snapshot-aged"}, nil), "us-east-1")

	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	f := got[0]
	if f.Metadata["size_is_instance_allocation"] == "true" {
		t.Fatalf("fell back to the instance size although the real size is known")
	}
	if want := 500.0 / 1024 * 0.095; math.Abs(f.MonthlyCost-want) > 0.0001 {
		t.Errorf("cost = $%.4f, want $%.4f", f.MonthlyCost, want)
	}
	if !strings.Contains(f.Reason, "500.0 MiB") {
		t.Errorf("reason hides the real size: %q", f.Reason)
	}
}

func TestRDSSnapshotFallsBackToInstanceSize(t *testing.T) {
	inv := zombie.Inventory{Now: now, DBSnapshots: []zombie.DBSnapshot{
		{ID: "snap-legacy", DBInstance: "prod", Type: "manual", Status: "available",
			StorageGiB: 100, CreatedAt: daysAgo(400)},
	}}

	got := price.Apply(detect.Run(inv, detect.Defaults(), []string{"rds-snapshot-aged"}, nil), "us-east-1")

	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	if !strings.Contains(got[0].CostBasis, "upper bound") {
		t.Errorf("fallback price not labelled: %q", got[0].CostBasis)
	}
	if math.Abs(got[0].MonthlyCost-100*0.095) > 0.0001 {
		t.Errorf("cost = $%.4f, want $9.50", got[0].MonthlyCost)
	}
}

func TestDBSnapshotCollectorKeepsExactBytes(t *testing.T) {
	var asked string
	api := &fake.RDS{
		DescribeDBSnapshotsFunc: func(_ context.Context, in *rds.DescribeDBSnapshotsInput) (*rds.DescribeDBSnapshotsOutput, error) {
			asked = aws.ToString(in.SnapshotType)
			return &rds.DescribeDBSnapshotsOutput{DBSnapshots: []rdstypes.DBSnapshot{{
				DBSnapshotIdentifier:    aws.String("snap-small"),
				AllocatedStorage:        aws.Int32(20),
				FullSnapshotSizeInBytes: aws.Int64(500 << 20),
			}}}, nil
		},
	}

	got, err := collect.DBSnapshots(context.Background(), api)
	if err != nil {
		t.Fatal(err)
	}
	// automated snapshots expire on their own; paging through them is waste
	if asked != "manual" {
		t.Errorf("SnapshotType = %q, want manual", asked)
	}
	if len(got) != 1 || got[0].ActualBytes != 500<<20 {
		t.Fatalf("got %+v, want ActualBytes = %d", got, 500<<20)
	}
}
