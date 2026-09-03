package cli

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/Sachinxmpl/zombie-scanner/awsapi"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/filter"
	"github.com/Sachinxmpl/zombie-scanner/render"
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

	format := o.Output
	if o.JSON {
		format = "json"
	}
	if format != "table" && format != "json" {
		return fmt.Errorf("unknown output format %q (want table or json)", format)
	}

	if err := validateDetectors(o.Only, o.Skip); err != nil {
		return err
	}

	filters := []filter.Filter{}
	if o.MinCost > 0 {
		filters = append(filters, filter.MinCost{USD: o.MinCost})
	}
	if o.Confidence != "" {
		level, err := zombie.ParseConfidence(o.Confidence)
		if err != nil {
			return err
		}
		filters = append(filters, filter.MinConfidence{Level: level})
	}

	logger, err := newLogger(cmd.ErrOrStderr(), o.LogLevel, o.Verbose)
	if err != nil {
		return err
	}

	cfg := detect.Defaults()
	cfg.SnapshotAgeDays = o.SnapshotAgeDays
	cfg.StoppedDays = o.StoppedDays
	cfg.IdleWindowDays = o.IdleWindowDays

	// nothing above here touches the network, i.e a bad flag never costs a
	// round trip and never reports itself as a credentials problem

	aws, err := awsapi.New(ctx, awsapi.Options{
		Profile: o.Profile,
		Region:  o.Region,
	})
	if err != nil {
		return err
	}

	eng := &scan.Engine{AWS: aws, Cfg: cfg, Filters: filters, Log: logger, Concurrency: o.Concurrency}

	opts := scan.Options{Only: o.Only, Skip: o.Skip, AllRegions: o.AllRegions}
	if o.Region != "" {
		opts.Regions = []string{o.Region}
	}

	report, err := eng.Run(ctx, opts)
	if err != nil {
		return err
	}

	report.Findings = render.Sort(report.Findings)
	report.Tool = zombie.ToolInfo{Name: "zombie-scanner", Version: o.version, Commit: o.commit}

	// report to stdout, diagnostics to stderr, so --json | jq stays clean
	w := cmd.OutOrStdout()
	if format == "json" {
		err = render.JSON(w, report)
	} else {
		err = render.Table(w, report, render.TableOptions{
			NoColor: o.NoColor,
			Verbose: o.Verbose,
		})
	}
	if err != nil {
		return err
	}

	render.Errors(cmd.ErrOrStderr(), report, o.Verbose)

	if o.Strict && len(report.Errors) > 0 {
		return fmt.Errorf("%d check(s) could not run and --strict is set", len(report.Errors))
	}
	if o.FailIfAbove > 0 && report.Summary.TotalMonthlyUSD > o.FailIfAbove {
		return ErrSpendAboveThreshold
	}

	return nil
}

// a typo in --only must not silently scan nothing
func validateDetectors(lists ...[]string) error {
	known := make(map[string]bool)
	for _, d := range detect.All() {
		known[d.Name()] = true
	}
	for _, list := range lists {
		for _, name := range list {
			if !known[name] {
				return fmt.Errorf("unknown detector %q (see `zombie-scanner detectors`)", name)
			}
		}
	}
	return nil
}

func newLogger(w io.Writer, level string, verbose bool) (*slog.Logger, error) {
	var l slog.Level
	if verbose {
		l = slog.LevelDebug
	} else if err := l.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("unknown log level %q (want debug, info, warn or error)", level)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: l})), nil
}
