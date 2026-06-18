package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(buildinfo.String())
		},
	}
}
