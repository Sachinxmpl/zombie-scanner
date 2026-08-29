package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/price"
	"github.com/Sachinxmpl/zombie-scanner/scan"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

// explicit alias for the bare invocation
func newScanCommand(o *options) *cobra.Command {
	return &cobra.Command{
		Use:           "scan",
		Short:         "Scan for zombie resources (the default action)",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runScan(cmd, *o)
		},
	}
}

func runScan(cmd *cobra.Command, o options) error {
	ctx := cmd.Context()

	aws, err := awsapi.New(ctx, awsapi.Options{
		Profile: o.Profile,
		Region:  o.Region,
	})
	if err != nil {
		return err
	}

	eng := &scan.Engine{AWS: aws, Cfg: detect.Defaults()}

	var opts scan.Options
	if o.Region != "" {
		opts.Regions = []string{o.Region}
	}

	report, err := eng.Run(ctx, opts)
	if err != nil {
		return err
	}

	// report to stdout, diagnostics to stderr, so --json | jq stays clean
	renderText(cmd.OutOrStdout(), report)
	renderErrors(cmd.ErrOrStderr(), report)
	return nil
}

func renderText(w io.Writer, r zombie.Report) {
	fmt.Fprintf(w, "Account %s, %d region(s)\n\n", r.AccountID, len(r.Regions))

	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "No zombies found. Clean account.")
		return
	}

	fmt.Fprintf(w, "%-22s %-12s %-11s %-7s %-8s %s\n",
		"RESOURCE", "TYPE", "REGION", "CONF", "~$/MO", "REASON")
	for _, f := range r.Findings {
		fmt.Fprintf(w, "%-22s %-12s %-11s %-7s $%-7.2f %s\n",
			f.ResourceID, f.ResourceType, f.Region, f.Confidence, f.MonthlyCost, f.Reason)
		fmt.Fprintf(w, "%-64s %s\n", "", f.CostBasis)
	}

	fmt.Fprintf(w, "\n%d zombie(s), estimated zombie spend ~$%.2f/month. Figures are estimates (rates updated %s)\n",
		r.Summary.ZombieCount, r.Summary.TotalMonthlyUSD, price.Updated())
}

func renderErrors(w io.Writer, r zombie.Report) {
	for _, e := range r.Errors {
		fmt.Fprintf(w, "warning: %s:%s in %s: %s (%s)\n",
			e.Service, e.Operation, e.Region, e.Message, e.Kind)
	}
	if len(r.Errors) > 0 {
		fmt.Fprintf(w, "%d check(s) skipped\n", len(r.Errors))
	}
}
