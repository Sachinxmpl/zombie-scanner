package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

const maxMarkdownRows = 100

type MarkdownOptions struct {
	Level int // heading level; 0 means "##"
	Rows  int // 0 means maxMarkdownRows
}

func Markdown(w io.Writer, r zombie.Report, o MarkdownOptions) error {
	r.Normalize()

	h := strings.Repeat("#", max(o.Level, 2))
	limit := o.Rows
	if limit <= 0 {
		limit = maxMarkdownRows
	}

	who := "account " + r.AccountID
	if len(r.Accounts) > 1 {
		who = fmt.Sprintf("%d accounts", len(r.Accounts))
	}

	fmt.Fprintf(w, "%s Zombie scan\n\n", h)

	if len(r.Findings) == 0 {
		if len(r.Filtered) > 0 {
			fmt.Fprintf(w, "No zombies matched your filters (%s).\n\n", filterSummary(r.Filtered))
		} else {
			fmt.Fprintf(w, "**No zombies found.** Clean %s.\n\n", who)
		}
		markdownErrors(w, r)
		markdownFooter(w, r, who)
		return nil
	}

	hidden := ""
	if len(r.Filtered) > 0 {
		hidden = " · " + filterSummary(r.Filtered)
	}
	fmt.Fprintf(w, "**%d zombie%s · estimated zombie spend ~$%.2f/month**%s\n\n",
		r.Summary.ZombieCount, plural(r.Summary.ZombieCount),
		r.Summary.TotalMonthlyUSD, hidden)

	multi := len(r.Accounts) > 1
	if multi {
		fmt.Fprintln(w, "| Resource | Account | Type | Region | Confidence | ~$/mo | Reason |")
		fmt.Fprintln(w, "| --- | --- | --- | --- | --- | --: | --- |")
	} else {
		fmt.Fprintln(w, "| Resource | Type | Region | Confidence | ~$/mo | Reason |")
		fmt.Fprintln(w, "| --- | --- | --- | --- | --: | --- |")
	}

	shown := min(len(r.Findings), limit)
	for _, f := range r.Findings[:shown] {
		cells := []string{"`" + mdCell(f.ResourceID) + "`"}
		if multi {
			cells = append(cells, mdCell(f.AccountID))
		}
		cells = append(cells,
			mdCell(f.ResourceType),
			mdCell(f.Region),
			f.Confidence.String(),
			fmt.Sprintf("$%.2f", f.MonthlyCost),
			mdCell(f.Reason),
		)
		fmt.Fprintf(w, "| %s |\n", strings.Join(cells, " | "))
	}
	fmt.Fprintln(w)

	if shown < len(r.Findings) {
		fmt.Fprintf(w, "_Showing the %d most expensive of %d findings. Run with `--output json` for all of them._\n\n",
			shown, len(r.Findings))
	}

	markdownErrors(w, r)
	markdownFooter(w, r, who)
	return nil
}

func markdownErrors(w io.Writer, r zombie.Report) {
	if len(r.Errors) == 0 {
		return
	}

	fmt.Fprintf(w, "<details>\n<summary>%d check%s skipped</summary>\n\n",
		len(r.Errors), plural(len(r.Errors)))
	fmt.Fprintln(w, "| Action | Region | Kind |")
	fmt.Fprintln(w, "| --- | --- | --- |")
	for _, e := range r.Errors {
		fmt.Fprintf(w, "| `%s` | %s | %s |\n",
			mdCell(e.Service+":"+e.Operation), mdCell(e.Region), e.Kind)
	}
	fmt.Fprintf(w, "\n</details>\n\n")
}

func markdownFooter(w io.Writer, r zombie.Report, who string) {
	regions := "no regions"
	if len(r.Regions) > 0 {
		regions = strings.Join(r.Regions, ", ")
	}
	fmt.Fprintf(w, "<sub>%s · %s · scanned %s · %s</sub>\n",
		who, mdCell(regions), r.ScannedAt.Format("2006-01-02 15:04 MST"), mdCell(estimateNote(r)))
}

var cellEscaper = strings.NewReplacer(
	"|", "\\|",
	"`", "'",
	"\r\n", " ",
	"\n", " ",
	"\r", " ",
)

func mdCell(s string) string {
	return cellEscaper.Replace(s)
}
