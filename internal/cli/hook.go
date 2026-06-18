package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/guard"
	"github.com/agenticraptor/agent-leash/internal/hook"
	"github.com/agenticraptor/agent-leash/internal/policy"
	"github.com/agenticraptor/agent-leash/internal/session"
	"github.com/agenticraptor/agent-leash/internal/shim"
)

func newHookCmd() *cobra.Command {
	var (
		format     string
		policyPath string
		workspace  string
	)
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Enforce the policy on a single tool call (reads a JSON event on stdin)",
		Long: `hook reads a "tool is about to be used" event as JSON on stdin and decides
whether to allow or deny it — the precise, per-action counterpart to run. It
understands Claude Code's PreToolUse payload and a generic {tool, input} schema,
so it works with any platform that can shell out on a tool call.

Wire it into Claude Code as a PreToolUse hook:

  {"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"agent-leash hook"}]}]}}

Output is JSON on stdout. With --format claude (the default) it speaks Claude
Code's permission-decision protocol; with --format generic it prints
{allow, decision, reason} and exits non-zero on deny.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			req, err := hook.Parse(data)
			if err != nil {
				return fmt.Errorf("parse hook payload: %w", err)
			}

			// Prefer an explicit --workspace; otherwise use the payload's cwd.
			ws := workspace
			if !cmd.Flags().Changed("workspace") && req.CWD != "" {
				ws = req.CWD
			}
			cfg, _, wsRoot, err := resolvePolicy(policyPath, ws)
			if err != nil {
				return err
			}

			res := hook.Evaluate(guard.New(cfg, wsRoot), req, cfg.Network.Allowed)

			// If this hook runs inside `agent-leash run`, record the decision so
			// the supervisor counts it and can react to denials.
			sessionDir := os.Getenv(shim.EnvSession)
			_ = session.Append(sessionDir, session.Event{
				Type:     session.TypeCommand,
				Command:  res.Command,
				Allow:    res.Allow,
				Category: res.Category,
				Reason:   res.Reason,
				Rule:     res.Rule,
				Action:   cfg.OnViolation.Action,
			})
			// If the payload reports a running cost, meter it so max_cost_usd works.
			if c := costField(req.Input); c > 0 {
				_ = session.Append(sessionDir, session.Event{Type: session.TypeCost, Cost: c})
			}

			var (
				body []byte
				code int
			)
			if format == "generic" {
				body, code = hook.GenericResponse(res, cfg.OnViolation.Action)
			} else {
				body, code = hook.ClaudeResponse(res, cfg.OnViolation.Action)
			}
			fmt.Fprintln(os.Stdout, string(body))
			if !res.Allow && cfg.OnViolation.Action != policy.ActionWarn {
				fmt.Fprintf(os.Stderr, "⛔ agent-leash: %s\n", res.Reason)
			}
			return exitError{code}
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&format, "format", "claude", "Output format: claude | generic")
	fl.StringVar(&policyPath, "policy", "", "Path to a policy file")
	fl.StringVarP(&workspace, "workspace", "C", ".", "Workspace root (defaults to the event's cwd)")
	return cmd
}

// costField extracts a reported running cost (USD) from a hook payload, if the
// platform includes one. JSON numbers decode to float64.
func costField(input map[string]any) float64 {
	for _, k := range []string{"total_cost_usd", "cost_usd", "cost", "usd"} {
		if v, ok := input[k]; ok {
			if f, ok := v.(float64); ok {
				return f
			}
		}
	}
	return 0
}
