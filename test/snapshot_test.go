package test

import (
	"slices"
	"testing"

	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// Deleting a snapshot an AMI depends on breaks the AMI. Cases 2 and 3 have
// identical snapshots and no images; only Failed differs, and the verdicts
// are opposite.
func TestSnapshotAgedRespectsWhatItCouldNotCheck(t *testing.T) {
	snaps := []zombie.Snapshot{
		{ID: "snap-in-use", StartedAt: daysAgo(400), SizeGiB: 50},
		{ID: "snap-orphan", StartedAt: daysAgo(400), SizeGiB: 50},
	}

	for _, tc := range []struct {
		name string
		inv  zombie.Inventory
		want []string
	}{
		{
			name: "cross-reference built: the AMI-backed snapshot is protected",
			inv: zombie.Inventory{Now: now, Snapshots: snaps,
				Images: []zombie.Image{{ID: "ami-1", SnapshotIDs: []string{"snap-in-use"}}}},
			want: []string{"snap-orphan"},
		},
		{
			name: "account genuinely has no AMIs: flag normally",
			inv:  zombie.Inventory{Now: now, Snapshots: snaps, Images: []zombie.Image{}},
			want: []string{"snap-in-use", "snap-orphan"},
		},
		{
			name: "DescribeImages failed: flag nothing",
			inv: zombie.Inventory{Now: now, Snapshots: snaps,
				Failed: map[string]bool{"ec2:DescribeImages": true}},
			want: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(detect.Run(tc.inv, detect.Defaults(), []string{"snapshot-aged"}, nil))
			if !slices.Equal(got, tc.want) {
				t.Errorf("flagged %v, want %v", got, tc.want)
			}
		})
	}
}
