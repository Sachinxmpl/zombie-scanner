package render

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/Sachinxmpl/zombie-scanner/zombie"
)

type TableOptions struct {
	NoColor bool
	Verbose bool // true -> show CostBasis under each row
	Width   int  // 0 = detect
}

const (
	idW   = 22
	typeW = 12
	regW  = 11
	confW = 7
	costW = 8
)

func Table(w io.Writer, r zombie.Report, o TableOptions) error {
	color := useColor(w, o.NoColor)

	width := o.Width
	if width == 0 {
		width = detectWidth(w)
	}

	dim := style(color, "240")
	head := style(color, "252").Bold(true)
	confStyle := map[zombie.Confidence]lipgloss.Style{
		zombie.High:   style(color, "203"),
		zombie.Medium: style(color, "214"),
		zombie.Low:    style(color, "245"),
	}

	fmt.Fprintf(w, "%s\n\n", dim.Render(fmt.Sprintf("Account %s · %s · scanned %s",
		r.AccountID, strings.Join(r.Regions, ", "),
		r.ScannedAt.Format("2006-01-02 15:04 MST"))))

	if len(r.Findings) == 0 {
		// "clean account" is a lie if a filter hid everything
		if len(r.Filtered) > 0 {
			fmt.Fprintf(w, "No zombies matched your filters (%s).\n", filterSummary(r.Filtered))
			return nil
		}
		fmt.Fprintln(w, "No zombies found. Clean account.")
		return nil
	}

	// REASON takes whatever width is left
	reasonW := width - (idW + typeW + regW + confW + costW + 5)
	if reasonW < 20 {
		reasonW = 20
	}

	fmt.Fprintln(w, head.Render(fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %s",
		idW, "RESOURCE", typeW, "TYPE", regW, "REGION",
		confW, "CONF", costW, "~$/MO", "REASON")))

	for _, f := range r.Findings {
		plain := f.Confidence.String()
		styled := plain
		if s, ok := confStyle[f.Confidence]; ok {
			styled = s.Render(plain)
		}

		fmt.Fprintf(w, "%-*s %-*s %-*s %s %-*s %s\n",
			idW, elide(f.ResourceID, idW),
			typeW, truncate(f.ResourceType, typeW),
			regW, truncate(f.Region, regW),
			pad(styled, plain, confW),
			costW, fmt.Sprintf("$%.2f", f.MonthlyCost),
			truncate(f.Reason, reasonW))

		if o.Verbose && f.CostBasis != "" {
			fmt.Fprintf(w, "%-*s %s\n", idW, "", dim.Render(f.CostBasis))
		}
	}

	hidden := ""
	if len(r.Filtered) > 0 {
		hidden = " · " + filterSummary(r.Filtered)
	}

	fmt.Fprintf(w, "\n%d zombie%s · estimated zombie spend ~$%.2f/month%s · %s\n",
		r.Summary.ZombieCount, plural(r.Summary.ZombieCount),
		r.Summary.TotalMonthlyUSD, hidden, dim.Render("figures are estimates"))

	return nil
}

// Errors writes partial failures. stderr only.
// Errors are grouped by kind and action: one missing permission across twenty regions is one line, not twenty.
func Errors(w io.Writer, r zombie.Report, verbose bool) {
	if len(r.Errors) == 0 {
		return
	}

	if verbose {
		for _, e := range r.Errors {
			fmt.Fprintf(w, "warning: %s:%s in %s: %s (%s)\n",
				e.Service, e.Operation, e.Region, e.Message, e.Kind)
		}
	}

	type group struct {
		kind    zombie.ErrorKind
		action  string
		message string
		regions []string
	}
	groups := map[string]*group{}
	order := []string{}

	for _, e := range r.Errors {
		// for AWS, service:Operation is the IAM action name
		action := e.Service + ":" + e.Operation
		k := string(e.Kind) + "|" + action
		g, ok := groups[k]
		if !ok {
			g = &group{kind: e.Kind, action: action, message: e.Message}
			groups[k] = g
			order = append(order, k)
		}
		g.regions = append(g.regions, e.Region)
	}

	fmt.Fprintf(w, "\n! %d check%s skipped\n", len(r.Errors), plural(len(r.Errors)))

	denied := false
	for _, k := range order {
		g := groups[k]
		switch g.kind {
		case zombie.KindAccessDenied:
			denied = true
			fmt.Fprintf(w, "  missing permission: %s (%s)\n", g.action, regionList(g.regions))
		case zombie.KindThrottled:
			fmt.Fprintf(w, "  throttled: %s (%s)\n", g.action, regionList(g.regions))
		case zombie.KindUnsupported:
			fmt.Fprintf(w, "  not available in %s: %s\n", regionList(g.regions), g.action)
		default:
			fmt.Fprintf(w, "  %s (%s): %s\n", g.action, regionList(g.regions), g.message)
		}
	}

	if denied {
		fmt.Fprintln(w, "  grant the missing actions, or skip those detectors with --skip")
		fmt.Fprintln(w, "  run `zombie-scanner iam-policy` for the policy this tool needs")
	}
	if !verbose {
		fmt.Fprintln(w, "  run with -v for detail")
	}
}

func regionList(regions []string) string {
	sort.Strings(regions)
	if len(regions) <= 2 {
		return strings.Join(regions, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(regions[:2], ", "), len(regions)-2)
}

func style(color bool, c string) lipgloss.Style {
	if !color {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c))
}

// explicit flag, then NO_COLOR, then "is anyone watching"
func useColor(w io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func detectWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 120
	}
	if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 40 {
		return cols
	}
	return 120
}

// elide keeps both ends: AWS ids share long prefixes
func elide(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	keep := max - 1
	head := (keep + 1) / 2
	return string(r[:head]) + "…" + string(r[len(r)-(keep-head):])
}

// truncate cuts prose at the end
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// pad against the plain string: ANSI escapes are not printable width
func pad(styled, plain string, w int) string {
	if n := w - len([]rune(plain)); n > 0 {
		return styled + strings.Repeat(" ", n)
	}
	return styled
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// sorted, because map iteration order would otherwise change between runs
func filterSummary(filtered map[string]int) string {
	parts := make([]string, 0, len(filtered))
	for name, n := range filtered {
		parts = append(parts, fmt.Sprintf("%d hidden by %s", n, name))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
