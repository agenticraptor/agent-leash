// Command agent-leash puts a hard budget and a kill-switch around an AI agent's
// actions — not just its tokens. It supervises any agent command, enforces
// limits on what the agent may do (files changed, new dependencies, commands
// run, wall-clock time, network, spend), and hard-stops the session the instant
// a limit is crossed, with a readable reason.
package main

import (
	"os"

	"github.com/agenticraptor/agent-leash/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
