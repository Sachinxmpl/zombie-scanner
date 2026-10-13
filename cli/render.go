package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Sachinxmpl/zombie-scanner/render"
	"github.com/Sachinxmpl/zombie-scanner/zombie"
	"github.com/spf13/cobra"
)

func newRenderCommand(o *options) *cobra.Command {
	return &cobra.Command{
		Use:   "render [report.json]",
		Short: "Re-render a saved JSON report in another format",
		Long: `Re-render a saved JSON report.

    zombie-scanner scan --output json > report.json
    zombie-scanner render report.json --output markdown

Reads stdin when no file is given. This makes no AWS calls.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := o.Output
			if o.JSON {
				format = "json"
			}

			in := cmd.InOrStdin()
			if len(args) == 1 && args[0] != "-" {
				fh, err := os.Open(args[0])
				if err != nil {
					return err
				}
				defer fh.Close()
				in = fh
			}

			var report zombie.Report
			if err := json.NewDecoder(in).Decode(&report); err != nil {
				return fmt.Errorf("reading report: %w", err)
			}
			report.Normalize()

			w := cmd.OutOrStdout()
			switch format {
			case "json":
				return render.JSON(w, report)
			case "markdown":
				return render.Markdown(w, report, render.MarkdownOptions{})
			case "table":
				return render.Table(w, report, render.TableOptions{
					NoColor: o.NoColor,
					Verbose: o.Verbose,
				})
			default:
				return fmt.Errorf("unknown output format %q (want table, json or markdown)", format)
			}
		},
	}
}
