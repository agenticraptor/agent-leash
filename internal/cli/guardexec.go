package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/shim"
)

// newGuardExecCmd is the hidden entrypoint the PATH shims call. The shim for a
// guarded binary runs `agent-leash guard-exec --name <binary> -- <args...>`,
// which evaluates the command and then either execs the real binary or blocks.
func newGuardExecCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:                "guard-exec --name <binary> -- <args...>",
		Short:              "Internal: evaluate and run an intercepted command (used by PATH shims)",
		Hidden:             true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: false,
		RunE: func(_ *cobra.Command, args []string) error {
			code := shim.RunGuardExec(name, args, os.Stderr)
			return exitError{code}
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "The intercepted binary name")
	return cmd
}
