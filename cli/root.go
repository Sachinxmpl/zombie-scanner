package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sachinxmpl/zombie-scanner/config"
	"github.com/Sachinxmpl/zombie-scanner/detect"
	"github.com/Sachinxmpl/zombie-scanner/filter"
)

// holds flag values for one command invocation
type options struct {
	Region     string
	AllRegions bool
	Profile    string

	NoColor bool
	Verbose bool
	Output  string
	JSON    bool

	Only, Skip []string
	MinCost    float64
	Confidence string

	SnapshotAgeDays int
	StoppedDays     int
	IdleWindowDays  int

	FailIfAbove float64
	Strict      bool
	LogLevel    string

	Concurrency int

	KeepTag   string
	NoKeepTag bool

	ConfigPath string
	NoConfig   bool

	config *config.File

	version, commit string

	TFState []string
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

	// flag defaults come from detect.Defaults(), so --help shows the real numbers
	d := detect.Defaults()
	pf := root.PersistentFlags()

	pf.StringVar(&o.Region, "region", "", "region to scan (default: the region the SDK resolves)")
	pf.BoolVar(&o.AllRegions, "all-regions", false, "scan every region the account has opted into")
	pf.StringVar(&o.Profile, "profile", "", "AWS profile to use")

	pf.StringVar(&o.Output, "output", "table", "output format: table|json")
	pf.BoolVar(&o.JSON, "json", false, "shorthand for --output json")
	pf.BoolVar(&o.NoColor, "no-color", false, "disable colour output")
	pf.BoolVarP(&o.Verbose, "verbose", "v", false, "show how each cost was calculated")

	pf.StringSliceVar(&o.Only, "only", nil, "run only these detectors")
	pf.StringSliceVar(&o.Skip, "skip", nil, "never run these detectors")
	pf.Float64Var(&o.MinCost, "min-cost", 0, "hide findings below this monthly cost")
	pf.StringVar(&o.Confidence, "confidence", "", "minimum confidence: LOW|MEDIUM|HIGH")

	pf.IntVar(&o.SnapshotAgeDays, "snapshot-age-days", d.SnapshotAgeDays, "flag snapshots older than this")
	pf.IntVar(&o.StoppedDays, "stopped-days", d.StoppedDays, "flag instances stopped longer than this")
	pf.IntVar(&o.IdleWindowDays, "idle-window-days", d.IdleWindowDays, "metric lookback window in days")

	pf.Float64Var(&o.FailIfAbove, "fail-if-above", 0, "exit 2 when monthly zombie spend exceeds this")
	pf.BoolVar(&o.Strict, "strict", false, "exit 1 if any detector fails to run")
	pf.StringVar(&o.LogLevel, "log-level", "info", "log level: debug|info|warn|error")

	pf.IntVar(&o.Concurrency, "concurrency", 8, "regions to scan in parallel")

	pf.StringVar(&o.KeepTag, "keep-tag", filter.DefaultKeepTag, "hide resources carrying this tag key")
	pf.BoolVar(&o.NoKeepTag, "no-keep-tag", false, "ignore the keep tag and show all findings")

	pf.StringVar(&o.ConfigPath, "config", "", "config file (default: ./"+config.FileName+", then $HOME)")
	pf.BoolVar(&o.NoConfig, "no-config", false, "ignore any config file")

	pf.StringSliceVar(&o.TFState, "tf-state", nil,
		"Terraform state file or directory; hides everything Terraform already manages")

	// --region and --all-regions are mutually exclusive
	root.MarkFlagsMutuallyExclusive("region", "all-regions")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := applyEnv(cmd); err != nil {
			return err
		}
		return applyConfig(cmd, &o)
	}

	root.AddCommand(
		newScanCommand(&o),
		newDetectorsCommand(),
		newVersionCommand(version, commit),
		newIAMPolicyCommand(&o),
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

// flag > environment > default
func applyEnv(cmd *cobra.Command) error {
	for flagName, envName := range map[string]string{
		"region":            "ZOMBIE_SCANNER_REGION",
		"profile":           "ZOMBIE_SCANNER_PROFILE",
		"output":            "ZOMBIE_SCANNER_OUTPUT",
		"min-cost":          "ZOMBIE_SCANNER_MIN_COST",
		"confidence":        "ZOMBIE_SCANNER_CONFIDENCE",
		"snapshot-age-days": "ZOMBIE_SCANNER_SNAPSHOT_AGE_DAYS",
		"stopped-days":      "ZOMBIE_SCANNER_STOPPED_DAYS",
		"idle-window-days":  "ZOMBIE_SCANNER_IDLE_WINDOW_DAYS",
		"keep-tag":          "ZOMBIE_SCANNER_KEEP_TAG",
	} {
		if cmd.Flags().Changed(flagName) {
			continue
		}
		v, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}
		if err := cmd.Flags().Set(flagName, v); err != nil {
			return fmt.Errorf("%s: %w", envName, err)
		}
	}
	return nil
}

// flag > environment > config file > default.
// Each tier skips a flag that an earlier tier already set
func applyConfig(cmd *cobra.Command, o *options) error {
	if o.NoConfig {
		return nil
	}

	var (
		f   *config.File
		err error
	)
	if o.ConfigPath != "" {
		f, err = config.Load(o.ConfigPath)
	} else {
		f, err = config.Discover()
	}
	if err != nil {
		return err
	}
	if f == nil {
		return nil
	}

	known := func(name string) bool {
		for _, d := range detect.All() {
			if d.Name() == name {
				return true
			}
		}
		return false
	}
	if err := filter.ValidateIgnoreRules(f.Ignore, known); err != nil {
		return fmt.Errorf("config %s: %w", f.Path, err)
	}

	for flagName, value := range f.FlagValues() {
		if cmd.Flags().Changed(flagName) {
			continue
		}
		if err := cmd.Flags().Set(flagName, value); err != nil {
			return fmt.Errorf("config %s: %s: %w", f.Path, flagName, err)
		}
	}

	o.config = f
	return nil
}
