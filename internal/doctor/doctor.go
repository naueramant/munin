package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Doctor runs diagnostics and formats reports.
type Doctor struct {
	opts Options
}

// New creates a new Doctor instance with the provided options.
func New(opts Options) *Doctor {
	return &Doctor{opts: opts}
}

// Run executes all diagnostic checks and compiles the report.
func (d *Doctor) Run() *Report {
	var allResults []CheckResult

	// 1. Dependencies
	allResults = append(allResults, checkDependencies()...)

	// 2. Systemd & Services
	allResults = append(allResults, checkSystemd(d.opts)...)

	// 3. Hardware & Display
	allResults = append(allResults, checkHardware()...)

	// 4. Configuration, Crontab & Git
	allResults = append(allResults, checkConfigAndGit(d.opts)...)

	summary := Summary{
		Total: len(allResults),
	}

	for _, r := range allResults {
		if r.FixApplied {
			summary.Fixed++
		}
		switch r.Status {
		case StatusOK:
			summary.Passed++
		case StatusWarn:
			summary.Warning++
		case StatusError:
			summary.Error++
		}
	}

	return &Report{
		Results: allResults,
		Summary: summary,
	}
}

// HasErrors returns true if any check produced a StatusError.
func (r *Report) HasErrors() bool {
	return r.Summary.Error > 0
}

// Render writes the diagnostic report to the given writer according to options.
func (d *Doctor) Render(w io.Writer, report *Report) error {
	if d.opts.JSON {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	useColor := isColorTerminal(w)
	printReportText(w, report, d.opts, useColor)
	return nil
}

func printReportText(w io.Writer, report *Report, opts Options, useColor bool) {
	fmt.Fprintln(w, "\nMunin System Diagnostics")
	fmt.Fprintln(w, "========================")

	// Group by category
	categories := []Category{
		CategoryDependencies,
		CategorySystemd,
		CategoryHardware,
		CategoryConfig,
		CategoryGit,
	}

	for _, cat := range categories {
		var catResults []CheckResult
		for _, r := range report.Results {
			if r.Category == cat {
				catResults = append(catResults, r)
			}
		}

		if len(catResults) == 0 {
			continue
		}

		fmt.Fprintf(w, "\n[%s]\n", cat)
		for _, r := range catResults {
			icon := colorizeStatus(r.Status, useColor)
			fmt.Fprintf(w, "  %s %s: %s\n", icon, r.Name, r.Message)

			if opts.Verbose && r.Detail != "" {
				fmt.Fprintf(w, "    Detail: %s\n", r.Detail)
			} else if !opts.Verbose && r.Status != StatusOK && r.Detail != "" {
				fmt.Fprintf(w, "    └─ Note: %s\n", r.Detail)
			}

			if r.FixApplied {
				fixText := "Auto-fix applied successfully"
				if useColor {
					fixText = "\033[32m" + fixText + "\033[0m"
				}
				fmt.Fprintf(w, "    └─ %s\n", fixText)
			} else if r.FixHint != "" && (r.Status == StatusWarn || r.Status == StatusError) {
				fmt.Fprintf(w, "    └─ Recommendation: %s\n", r.FixHint)
			}
		}
	}

	fmt.Fprintln(w)
	printSummaryLine(w, report.Summary, useColor)
	fmt.Fprintln(w)
}

func printSummaryLine(w io.Writer, s Summary, useColor bool) {
	if s.Error == 0 && s.Warning == 0 {
		msg := "All system checks passed! Munin is ready."
		if useColor {
			msg = "\033[1;32m" + msg + "\033[0m"
		}
		fmt.Fprintf(w, "Summary: %s (%d checks)\n", msg, s.Total)
		return
	}

	parts := []string{}
	if s.Passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", s.Passed))
	}
	if s.Warning > 0 {
		txt := fmt.Sprintf("%d warning(s)", s.Warning)
		if useColor {
			txt = "\033[33m" + txt + "\033[0m"
		}
		parts = append(parts, txt)
	}
	if s.Error > 0 {
		txt := fmt.Sprintf("%d error(s)", s.Error)
		if useColor {
			txt = "\033[1;31m" + txt + "\033[0m"
		}
		parts = append(parts, txt)
	}
	if s.Fixed > 0 {
		txt := fmt.Sprintf("%d fixed", s.Fixed)
		if useColor {
			txt = "\033[32m" + txt + "\033[0m"
		}
		parts = append(parts, txt)
	}

	summaryStr := ""
	for i, p := range parts {
		if i > 0 {
			summaryStr += ", "
		}
		summaryStr += p
	}

	fmt.Fprintf(w, "Summary: %s across %d total checks.\n", summaryStr, s.Total)
	if s.Error > 0 {
		fmt.Fprintln(w, "Please address the errors above before running Munin.")
	}
}

func colorizeStatus(s Status, useColor bool) string {
	sym := s.Symbol()
	if !useColor {
		return sym
	}

	switch s {
	case StatusOK:
		return "\033[32m" + sym + "\033[0m" // Green
	case StatusWarn:
		return "\033[33m" + sym + "\033[0m" // Yellow
	case StatusError:
		return "\033[1;31m" + sym + "\033[0m" // Bold Red
	case StatusInfo:
		return "\033[36m" + sym + "\033[0m" // Cyan
	default:
		return sym
	}
}

func isColorTerminal(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
