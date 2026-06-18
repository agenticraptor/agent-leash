package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/policy"
)

func newInitCmd() *cobra.Command {
	var global bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a documented starter policy file",
		Long: `init creates a commented .agent-leash.toml in the current directory (or the
user-level policy with --global) so you can tighten the defaults. It never
overwrites an existing file.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			var (
				path string
				err  error
			)
			if global {
				var target string
				if target, err = policy.XDGPath(); err != nil {
					return err
				}
				path, err = policy.InitFile(target)
			} else {
				path, err = policy.Init(".")
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Wrote %s\n", path)
			fmt.Fprintln(os.Stdout, "Edit it to tighten limits, then run: agent-leash run -- <your agent>")
			return nil
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "Write the user-level policy (XDG) instead of a project file")
	return cmd
}
