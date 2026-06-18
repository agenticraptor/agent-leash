package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/session"
	"github.com/agenticraptor/agent-leash/internal/shim"
)

func newReportCmd() *cobra.Command {
	var cost float64
	cmd := &cobra.Command{
		Use:   "report --cost <usd>",
		Short: "Report spend to the supervising session (enables max_cost_usd)",
		Long: `report tells the supervising 'agent-leash run' how much has been spent so far,
so the max_cost_usd limit can be enforced. Call it from your agent or a wrapper
whenever you have an updated cost — each value is added to the session total.

It is a no-op (exit 0) when not running inside 'agent-leash run', so it is safe
to call unconditionally:

  agent-leash report --cost 0.12`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("cost") {
				return cobraError("provide a value, e.g. --cost 0.12")
			}
			if cost < 0 {
				return cobraError("--cost must not be negative")
			}
			dir := os.Getenv(shim.EnvSession)
			if dir == "" {
				fmt.Fprintln(os.Stderr, "agent-leash: not inside a leashed session; nothing to report")
				return nil
			}
			return session.Append(dir, session.Event{Type: session.TypeCost, Cost: cost})
		},
	}
	cmd.Flags().Float64Var(&cost, "cost", 0, "Spend to add to the session total, in USD")
	return cmd
}
