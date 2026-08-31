package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sachinxmpl/zombie-scanner/detect"
)

// holds flag values for one command invocation
type options struct {
	Region  string
	Profile string
	NoColor bool
	Verbose bool
	Output  string
	JSON    bool

	version, commit string
}

// builds the command tree
func NewRootCommand(version, commit string) *cobra.Command {
	o := options{version: version, commit: commit}

	root := &cobra.Command{
		Use:   "zombie-scanner",
		Short: "Find AWS resources that are dead but still billing",
		Long: `zombie-scanner reads your AWS account through read-only APIs and reports
resources that cost money while nothing uses them.

It never creates, modifies, or deletes anything.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,

		RunE: func(cmd *cobra.Command, _ []string) error {
			return runScan(cmd, o)
		},
	}

	root.PersistentFlags().StringVar(&o.Region, "region", "", "region to scan (default: the region the SDK resolves)")
	root.PersistentFlags().StringVar(&o.Profile, "profile", "", "AWS profile to use")
	root.PersistentFlags().BoolVar(&o.NoColor, "no-color", false, "disable colour output")
	root.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "show how each cost was calculated")
	root.PersistentFlags().StringVar(&o.Output, "output", "table", "output format: table|json")
	root.PersistentFlags().BoolVar(&o.JSON, "json", false, "shorthand for --output json")

	root.AddCommand(
		newScanCommand(&o),
		newDetectorsCommand(),
		newVersionCommand(version, commit),
	)

	return root
}

func newDetectorsCommand() *cobra.Command {
	return &cobra.Command{
		Use:           "detectors",
		Short:         "List the available detectors",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			for _, d := range detect.All() {
				fmt.Fprintf(w, "%-20s %s\n", d.Name(), d.Describe())
				fmt.Fprintf(w, "%-20s needs: %v\n\n", "", d.Needs())
			}
			return nil
		},
	}
}

func newVersionCommand(version, commit string) *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "Print the version",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "zombie-scanner %s (%s)\n", version, commit)
			return nil
		},
	}
}
