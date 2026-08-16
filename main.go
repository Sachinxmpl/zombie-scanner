package main

import (
	"fmt"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func main() {
	// Sample inventory (actual implementation later)

	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	inv := zombie.Inventory{
		Region:    "us-east-1",
		AccountID: "123456789012",
		Now:       now,
		Volumes: []zombie.Volume{
			{
				ID: "vol-0abc123def456789", State: "available",
				VolumeType: "gp3", SizeGiB: 100,
				CreatedAt: now.AddDate(0, 0, -45),
			},
			{
				ID: "vol-0deadbeef00000001", State: "in-use",
				VolumeType: "gp2", SizeGiB: 8,
				CreatedAt: now.AddDate(0, 0, -200), AttachedTo: "i-0123456789abcdef",
			},
			{
				ID: "vol-0freshdetached002", State: "available",
				VolumeType: "gp3", SizeGiB: 500,
				CreatedAt: now.Add(-2 * time.Hour), // too new to judge
			},
		},
	}

	findings := detect.Run(inv, detect.Defaults(), nil, nil)

	fmt.Printf("Scanned %s %d volumes, %d detector(s) registered\n\n",
		inv.Region, len(inv.Volumes), len(detect.All()))

	fmt.Printf("%-22s %-12s %-11s %-7s %s\n", "RESOURCE", "TYPE", "REGION", "CONF", "REASON")

	for _, f := range findings {
		fmt.Printf("%-22s %-12s %-11s %-7s %s\n", f.ResourceID, f.ResourceType, f.Region, f.Confidence, f.Reason)
	}
	fmt.Printf("\n%d zombie(s) found\n", len(findings))
}
