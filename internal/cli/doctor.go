package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/agenticraptor/agent-leash/internal/buildinfo"
	"github.com/agenticraptor/agent-leash/internal/guard"
	"github.com/agenticraptor/agent-leash/internal/shim"
)

func newDoctorCmd() *cobra.Command {
	var (
		policyPath string
		workspace  string
	)
	cmd := &cobra.Command{
		Use:           "doctor",
		Short:         "Check the environment, policy, and enforcement capabilities",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			runDoctor(policyPath, workspace)
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&policyPath, "policy", "", "Path to a policy file")
	fl.StringVarP(&workspace, "workspace", "C", ".", "Workspace root to check")
	return cmd
}

func runDoctor(policyPath, workspace string) {
	ok := func(label, detail string) { fmt.Printf("  \033[32m✓\033[0m %-20s %s\n", label, detail) }
	warn := func(label, detail string) { fmt.Printf("  \033[33m!\033[0m %-20s %s\n", label, detail) }
	bad := func(label, detail string) { fmt.Printf("  \033[31m✗\033[0m %-20s %s\n", label, detail) }

	fmt.Printf("%s\n\n", buildinfo.String())
	ok("platform", runtime.GOOS+"/"+runtime.GOARCH)

	cfg, path, wsRoot, err := resolvePolicy(policyPath, workspace)
	if err != nil {
		bad("policy", err.Error())
		return
	}
	if path != "" {
		ok("policy", path)
	} else {
		warn("policy", "built-in defaults (run `agent-leash init` to customize)")
	}
	ok("workspace", wsRoot)

	if cfg.Network.Allowed {
		warn("network", "allowed (set network.allowed = false to block it)")
	} else {
		ok("network", "blocked by policy")
	}

	g := guard.New(cfg, wsRoot)
	ok("guarded commands", fmt.Sprintf("%d (e.g. %s)", len(g.GuardSet()), preview(g.GuardSet(), 6)))

	// Can we install PATH shims? (run-mode command interception)
	if dir, err := os.MkdirTemp("", "agent-leash-doctor-"); err == nil {
		defer os.RemoveAll(dir)
		if err := shim.Install("agent-leash", filepath.Join(dir, "bin"), []string{"rm"}); err == nil {
			ok("command shims", "can intercept commands under `run`")
		} else {
			warn("command shims", "could not write shims: "+err.Error())
		}
	}

	// OS hardening availability.
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("unshare"); err == nil {
			ok("os hardening", "`unshare` available for --harden (network namespace)")
		} else {
			warn("os hardening", "`unshare` not found; --harden falls back to command guards")
		}
	default:
		warn("os hardening", "OS network isolation is Linux-only; command guards still apply")
	}

	fmt.Print("\nTip: start a leashed session with `agent-leash run -- <your agent>`.\n")
}

func preview(items []string, n int) string {
	if len(items) > n {
		items = items[:n]
	}
	out := ""
	for i, it := range items {
		if i > 0 {
			out += " "
		}
		out += it
	}
	return out
}
