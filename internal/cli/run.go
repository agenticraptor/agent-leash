package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/budget"
	"github.com/agenticraptor/agent-leash/internal/policy"
	"github.com/agenticraptor/agent-leash/internal/report"
	"github.com/agenticraptor/agent-leash/internal/supervisor"
)

type runFlags struct {
	policy      string
	workspace   string
	harden      bool
	status      bool
	quiet       bool
	noNetwork   bool
	network     bool
	maxFiles    int
	maxDeps     int
	maxCommands int
	maxDuration string
	maxCost     float64
}

func newRunCmd() *cobra.Command {
	var f runFlags
	cmd := &cobra.Command{
		Use:   "run [flags] -- <command> [args...]",
		Short: "Supervise an agent command and enforce the policy live",
		Long: `run launches your agent (or any command) under the policy and watches it as
it works. It hard-stops the whole process group the moment a limit is crossed.

Everything after -- is the command to run, for example:

  agent-leash run -- claude
  agent-leash run --no-network --max-files 10 -- aider
  agent-leash run --harden -- ./my-agent --task "refactor"`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cobraError("nothing to run — pass the agent command after --, e.g. `agent-leash run -- claude`")
			}
			return nil
		},
		DisableFlagParsing: false,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, &f, args)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.policy, "policy", "", "Path to a policy file (default: discovered .agent-leash.toml or built-in)")
	fl.StringVarP(&f.workspace, "workspace", "C", ".", "Workspace root the agent is confined to")
	fl.BoolVar(&f.harden, "harden", false, "Add OS-level isolation where available (Linux network namespace)")
	fl.BoolVar(&f.status, "status", false, "Print a live status line while the agent runs")
	fl.BoolVarP(&f.quiet, "quiet", "q", false, "Suppress the startup banner and session report")
	fl.BoolVar(&f.noNetwork, "no-network", false, "Force network access off for this run")
	fl.BoolVar(&f.network, "network", false, "Force network access on for this run")
	fl.IntVar(&f.maxFiles, "max-files", 0, "Override the max files-changed limit (0 = use policy)")
	fl.IntVar(&f.maxDeps, "max-deps", 0, "Override the max new-dependencies limit")
	fl.IntVar(&f.maxCommands, "max-commands", 0, "Override the max guarded-commands limit")
	fl.StringVar(&f.maxDuration, "max-duration", "", "Override the wall-clock limit, e.g. 20m")
	fl.Float64Var(&f.maxCost, "max-cost", 0, "Override the spend limit in USD")
	return cmd
}

func runRun(cmd *cobra.Command, f *runFlags, command []string) error {
	cfg, path, wsRoot, err := resolvePolicy(f.policy, f.workspace)
	if err != nil {
		return err
	}
	if err := applyOverrides(cmd, f, &cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	out := os.Stderr
	if !f.quiet {
		report.Startup(out, wsRoot, path, cfg)
	}

	opt := supervisor.Options{
		Command:    command,
		Policy:     cfg,
		PolicyPath: path,
		Workspace:  wsRoot,
		Harden:     f.harden,
		Now:        time.Now,
		Logf: func(format string, args ...any) {
			if !f.quiet {
				fmt.Fprintf(out, "   %s\n", fmt.Sprintf(format, args...))
			}
		},
		OnStop: func(c supervisor.StopCause) { report.StopBanner(out, c) },
		OnStatus: func(s budget.Snapshot) {
			if f.status {
				report.StatusLine(out, s, cfg.Limits)
			}
		},
	}

	res, err := supervisor.Run(cmd.Context(), opt)
	if err != nil {
		return err
	}
	if f.status {
		fmt.Fprintln(out)
	}
	if !f.quiet {
		report.SessionReport(out, res, cfg.Limits)
	}
	if res.ExitCode != 0 {
		return exitError{res.ExitCode}
	}
	return nil
}

// applyOverrides mutates cfg with any command-line limit overrides that were set.
func applyOverrides(cmd *cobra.Command, f *runFlags, cfg *policy.Config) error {
	ch := cmd.Flags().Changed
	if ch("no-network") && f.noNetwork {
		cfg.Network.Allowed = false
	}
	if ch("network") && f.network {
		cfg.Network.Allowed = true
	}
	if ch("max-files") {
		cfg.Limits.MaxFilesChanged = f.maxFiles
	}
	if ch("max-deps") {
		cfg.Limits.MaxNewDeps = f.maxDeps
	}
	if ch("max-commands") {
		cfg.Limits.MaxCommands = f.maxCommands
	}
	if ch("max-cost") {
		cfg.Limits.MaxCostUSD = f.maxCost
	}
	if ch("max-duration") {
		d, err := time.ParseDuration(f.maxDuration)
		if err != nil {
			return fmt.Errorf("--max-duration: %w", err)
		}
		cfg.Limits.MaxDuration = policy.Duration{Duration: d}
	}
	return nil
}
