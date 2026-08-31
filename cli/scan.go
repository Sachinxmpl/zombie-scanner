package cli

import (
	"github.com/spf13/cobra"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/render"
	"github.com/Sachinxmpl/zombie-scanner/scan"
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

	report.Findings = render.Sort(report.Findings)

	// report to stdout, diagnostics to stderr, so --json | jq stays clean
	if err := render.Table(cmd.OutOrStdout(), report, render.TableOptions{
		NoColor: o.NoColor,
		Verbose: o.Verbose,
	}); err != nil {
		return err
	}
	render.Errors(cmd.ErrOrStderr(), report)

	return nil
}
