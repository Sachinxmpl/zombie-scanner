// Command zombie-scanner finds AWS resources that are dead but still billing.
// It is strictly read-only: no code path in this program creates, modifies, or
// deletes any AWS resource.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/scan"
)

func main() {
	ctx := context.Background()

	aws, err := awsapi.New(ctx, awsapi.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	eng := &scan.Engine{
		AWS: aws,
		Cfg: detect.Defaults(),
	}

	report, err := eng.Run(ctx, scan.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("Account %s, %d region(s), %d detector(s)\n\n",
		report.AccountID, len(report.Regions), len(detect.All()))

	if len(report.Findings) == 0 {
		fmt.Println("No zombies found. Clean account.")
	} else {
		fmt.Printf("%-22s %-12s %-11s %-7s %-8s %s\n",
			"RESOURCE", "TYPE", "REGION", "CONF", "~$/MO", "REASON")
		for _, f := range report.Findings {
			fmt.Printf("%-22s %-12s %-11s %-7s $%-7.2f %s\n",
				f.ResourceID, f.ResourceType, f.Region, f.Confidence, f.MonthlyCost, f.Reason)
			fmt.Printf("%-64s %s\n", "", f.CostBasis)
		}
	}

	// Partial failures go to stderr so stdout stays pipeable.
	for _, e := range report.Errors {
		fmt.Fprintf(os.Stderr, "warning: %s:%s in %s: %s (%s)\n",
			e.Service, e.Operation, e.Region, e.Message, e.Kind)
	}

	fmt.Printf("\n%d zombie(s), estimated zombie spend ~$%.2f/month. Figures are estimates (rates updated %s)\n",
		report.Summary.ZombieCount, report.Summary.TotalMonthlyUSD, price.Updated())

	if len(report.Errors) > 0 {
		fmt.Printf("%d check(s) skipped, see warnings above\n", len(report.Errors))
	}
}
