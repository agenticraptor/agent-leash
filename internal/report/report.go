// Package report renders agent-leash's human-facing output: the startup banner,
// the live status line, the stop banner shown when a limit trips, and the
// end-of-session report card.
package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agenticraptor/agent-leash/internal/budget"
	"github.com/agenticraptor/agent-leash/internal/policy"
	"github.com/agenticraptor/agent-leash/internal/supervisor"
)

// Startup prints the one-line banner shown when a leashed session begins.
func Startup(w io.Writer, workspace, policyPath string, cfg policy.Config) {
	src := "built-in defaults"
	if policyPath != "" {
		src = policyPath
	}
	net := styleOK.Render("on")
	if !cfg.Network.Allowed {
		net = styleWarn.Render("off")
	}
	fmt.Fprintln(w, styleAccent.Render("🐕 agent-leash")+styleDim.Render(" — leashing this session"))
	fmt.Fprintf(w, "   %s %s\n", styleLabel.Render("workspace"), workspace)
	fmt.Fprintf(w, "   %s %s\n", styleLabel.Render("policy   "), src)
	fmt.Fprintf(w, "   %s %s\n", styleLabel.Render("limits   "), limitsSummary(cfg.Limits))
	fmt.Fprintf(w, "   %s network %s · on violation: %s\n\n",
		styleLabel.Render("guards   "), net, styleWarn.Render(cfg.OnViolation.Action))
}

// StatusLine prints a compact, overwriting status line for --status.
func StatusLine(w io.Writer, s budget.Snapshot, lim policy.Limits) {
	parts := []string{
		fmt.Sprintf("files %s", ratio(s.FilesChanged, lim.MaxFilesChanged)),
		fmt.Sprintf("deps %s", ratio(s.NewDeps, lim.MaxNewDeps)),
		fmt.Sprintf("cmds %s", count(s.Commands, lim.MaxCommands)),
		fmt.Sprintf("time %s", durRatio(s.Elapsed, lim.MaxDuration.Duration)),
	}
	if lim.MaxCostUSD > 0 {
		parts = append(parts, fmt.Sprintf("cost $%.2f/$%.2f", s.CostUSD, lim.MaxCostUSD))
	}
	fmt.Fprintf(w, "\r%s %s", styleAccent.Render("🐕 leash"), styleDim.Render(strings.Join(parts, " · ")))
}

// StopBanner prints the box shown the moment a limit is crossed.
func StopBanner(w io.Writer, c supervisor.StopCause) {
	title := styleDanger.Render("⛔  agent-leash stopped the session")
	body := title + "\n" + c.Reason
	if c.Kind == "command" && c.Detail != "" {
		body += "\n" + styleDim.Render("$ "+c.Detail)
	}
	fmt.Fprintln(w, "\n"+stopBox.Render(body)+"\n")
}

// SessionReport prints the end-of-session card summarizing usage against limits.
func SessionReport(w io.Writer, res supervisor.Result, lim policy.Limits) {
	var b strings.Builder
	header := styleAccent.Render("agent-leash") + styleDim.Render(" · session report")
	fmt.Fprint(&b, header+"\n")

	status := styleOK.Render("completed within budget")
	if res.Stopped {
		status = styleDanger.Render("STOPPED") + " — " + res.Cause.Reason
	}
	fmt.Fprint(&b, line("status", status))
	fmt.Fprint(&b, line("files changed", ratio(res.Usage.FilesChanged, lim.MaxFilesChanged)))
	fmt.Fprint(&b, line("new deps", ratio(res.Usage.NewDeps, lim.MaxNewDeps)))
	fmt.Fprint(&b, line("commands", count(res.Usage.Commands, lim.MaxCommands)))
	fmt.Fprint(&b, line("time", durRatio(res.Usage.Elapsed, lim.MaxDuration.Duration)))
	if lim.MaxCostUSD > 0 || res.Usage.CostUSD > 0 {
		fmt.Fprint(&b, line("cost", fmt.Sprintf("$%.2f / %s", res.Usage.CostUSD, dollars(lim.MaxCostUSD))))
	}
	if len(res.NewDeps) > 0 {
		fmt.Fprint(&b, line("added", summarize(res.NewDeps, 4)))
	}
	fmt.Fprint(&b, line("exit code", fmt.Sprintf("%d", res.ExitCode)))

	fmt.Fprintln(w, reportBox.Render(strings.TrimRight(b.String(), "\n")))
}

// Decision prints the result of `agent-leash check`.
func Decision(w io.Writer, allow bool, reason, command string) {
	if allow {
		fmt.Fprintf(w, "%s %s\n", styleOK.Render("✓ allow"), command)
		return
	}
	fmt.Fprintf(w, "%s %s\n   %s\n", styleDanger.Render("⛔ deny"), command, styleDim.Render(reason))
}

func line(label, value string) string {
	return fmt.Sprintf("%s  %s\n", styleLabel.Render(pad(label, 14)), value)
}

func limitsSummary(l policy.Limits) string {
	return fmt.Sprintf("files≤%s · deps≤%s · cmds≤%s · time≤%s · spend≤%s",
		limit(l.MaxFilesChanged), limit(l.MaxNewDeps), limit(l.MaxCommands),
		durLimit(l.MaxDuration.Duration), dollars(l.MaxCostUSD))
}

func ratio(cur, max int) string {
	if max <= 0 {
		return fmt.Sprintf("%d / ∞", cur)
	}
	s := fmt.Sprintf("%d / %d", cur, max)
	if cur > max {
		return styleDanger.Render(s)
	}
	return s
}

func count(cur, max int) string {
	if max <= 0 {
		return fmt.Sprintf("%d", cur)
	}
	return ratio(cur, max)
}

func durRatio(cur, max time.Duration) string {
	if max <= 0 {
		return formatDur(cur)
	}
	s := fmt.Sprintf("%s / %s", formatDur(cur), formatDur(max))
	if cur > max {
		return styleDanger.Render(s)
	}
	return s
}

func limit(n int) string {
	if n <= 0 {
		return "∞"
	}
	return fmt.Sprintf("%d", n)
}

func durLimit(d time.Duration) string {
	if d <= 0 {
		return "∞"
	}
	return formatDur(d)
}

func dollars(v float64) string {
	if v <= 0 {
		return "∞"
	}
	return fmt.Sprintf("$%.2f", v)
}

func formatDur(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func summarize(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(" (+%d more)", len(items)-max)
}
