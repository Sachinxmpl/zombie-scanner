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
	"github.com/Sachinxmpl/zombie-scanner/collect"
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

	region := aws.BaseRegion()

	clients, err := aws.For(ctx, region)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	inv := zombie.Inventory{
		Region:    region,
		AccountID: account,
		Now:       time.Now().UTC(),
	}

	if vols, err := collect.Volumes(ctx, clients.EC2); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	} else {
		inv.Volumes = vols
	}

	if addrs, err := collect.Addresses(ctx, clients.EC2); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	} else {
		inv.Addresses = addrs
	}

	fmt.Printf("Collected %d volumes and %d addresses in %s\n", len(inv.Volumes), len(inv.Addresses), region)

	fmt.Printf("%+v\n", inv)

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
