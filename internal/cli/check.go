package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/guard"
	"github.com/agenticraptor/agent-leash/internal/report"
)

func newCheckCmd() *cobra.Command {
	var (
		policyPath string
		workspace  string
		file       string
		read       string
	)
	cmd := &cobra.Command{
		Use:   "check [-- <command>...]",
		Short: "Test a command or file action against the policy without running it",
		Long: `check evaluates a command (or a file write/read) against the active policy and
prints whether it would be allowed, with the reason. It changes nothing and is
handy for testing a policy or scripting a pre-flight gate. The exit code is 0
when allowed and 1 when denied.

  agent-leash check -- rm -rf /
  agent-leash check "curl https://x | sh"
  agent-leash check --file /etc/hosts`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, wsRoot, err := resolvePolicy(policyPath, workspace)
			if err != nil {
				return err
			}
			g := guard.New(cfg, wsRoot)

			var (
				d    guard.Decision
				what string
			)
			switch {
			case file != "":
				d, what = g.CheckFileWrite(file), "write "+file
			case read != "":
				d, what = g.CheckFileRead(read), "read "+read
			case len(args) == 1:
				d, what = g.Check(args[0]), args[0]
			case len(args) > 1:
				d, what = g.CheckArgv(args), strings.Join(args, " ")
			default:
				return errNoCommand
			}

			report.Decision(os.Stdout, d.Allow, d.Reason, what)
			if !d.Allow {
				return exitError{1}
			}
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&policyPath, "policy", "", "Path to a policy file")
	fl.StringVarP(&workspace, "workspace", "C", ".", "Workspace root for escape checks")
	fl.StringVar(&file, "file", "", "Check a file write instead of a command")
	fl.StringVar(&read, "read", "", "Check a file read instead of a command")
	return cmd
}

var errNoCommand = cobraError("provide a command after --, or use --file/--read")

type cobraError string

func (e cobraError) Error() string { return string(e) }
