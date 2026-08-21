// Command zombie-scanner finds AWS resources that are dead but still billing.
// It is strictly read-only: no code path in this program creates, modifies, or
// deletes any AWS resource.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

func main() {
	ctx := context.Background()

	aws, err := awsapi.New(ctx, awsapi.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// GetCallerIdentity needs no IAM permission
	// so a failure here means no credentials — every other call would fail too.
	account, err := aws.AccountID(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// Informational only
	regions, err := aws.Regions(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}

	fmt.Printf("Connected to AWS\n")
	fmt.Printf("  account         %s\n", account)
	fmt.Printf("  default region  %s\n", aws.BaseRegion())
	fmt.Printf("  regions enabled %d\n\n", len(regions))

	now := time.Now().UTC()

	inv := zombie.Inventory{
		Region:    aws.BaseRegion(),
		AccountID: account,
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

	findings := price.Apply(
		detect.Run(inv, detect.Defaults(), nil, nil),
		inv.Region,
	)

	fmt.Printf("Scanned %s, %d volumes, %d detector(s) registered\n\n",
		inv.Region, len(inv.Volumes), len(detect.All()))

	fmt.Printf("%-22s %-12s %-11s %-7s %-8s %s\n",
		"RESOURCE", "TYPE", "REGION", "CONF", "~$/MO", "REASON")

	total := 0.0
	for _, f := range findings {
		total += f.MonthlyCost
		fmt.Printf("%-22s %-12s %-11s %-7s $%-7.2f %s\n",
			f.ResourceID, f.ResourceType, f.Region, f.Confidence, f.MonthlyCost, f.Reason)
		fmt.Printf("%-64s %s\n", "", f.CostBasis)
	}

	fmt.Printf("\n%d zombie(s) found, estimated zombie spend ~$%.2f/month. Figures are estimates (rates updated %s)\n",
		len(findings), total, price.Updated())
}
