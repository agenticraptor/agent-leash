// Package cli wires the agent-leash command-line interface together.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/buildinfo"
	"github.com/agenticraptor/agent-leash/internal/policy"
)

// exitError carries a specific process exit code up to Execute.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// Execute runs the root command and returns a process exit code.
func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := newRootCmd().ExecuteContext(ctx)
	var ee exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-leash",
		Short: "A hard budget + kill-switch for your AI agent's actions, not just its tokens",
		Long: `agent-leash puts a leash on what an AI agent may do in a session — how many
files it changes, how many dependencies it adds, how long it runs, how much it
spends, whether it touches the network, and which commands it may never run —
and hard-stops it the instant a limit is crossed, with a readable reason.

Two ways to use it:

  agent-leash run -- <your agent command>   supervise any agent/CLI live
  agent-leash hook                          enforce per tool-call (Claude Code, etc.)

It works with zero config: run it and a protective default policy applies. Drop
an .agent-leash.toml in your project to tighten or loosen the rules.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Version,
	}
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	cmd.AddCommand(
		newRunCmd(),
		newCheckCmd(),
		newHookCmd(),
		newReportCmd(),
		newInitCmd(),
		newDoctorCmd(),
		newGuardExecCmd(),
		newVersionCmd(),
	)
	return cmd
}

// resolvePolicy loads the active policy starting discovery from the workspace
// directory and returns the config, the file path it came from (empty for
// built-in defaults), and the absolute workspace root.
func resolvePolicy(policyPath, workspace string) (policy.Config, string, string, error) {
	if workspace == "" {
		workspace = "."
	}
	wsAbs, err := filepath.Abs(workspace)
	if err != nil {
		return policy.Config{}, "", "", err
	}
	cfg, path, err := policy.LoadAuto(policyPath, wsAbs)
	if err != nil {
		return cfg, path, "", err
	}
	wsRoot, err := cfg.WorkspaceRoot(wsAbs)
	if err != nil {
		return cfg, path, "", err
	}
	return cfg, path, wsRoot, nil
}
