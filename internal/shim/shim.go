// Package shim installs lightweight executables on the agent's PATH that
// intercept guarded commands and route them through agent-leash before they
// run. When the agent invokes, say, `rm`, the shim named `rm` runs first and
// calls `agent-leash guard-exec`, which evaluates the command against the
// active policy and then either executes the real binary or blocks it.
//
// This is defense-in-depth, not a kernel jail: it covers commands an agent
// runs through the shell's PATH (the overwhelming majority), and is paired with
// the workspace watcher, the budget meter, and optional OS hardening for the
// cases PATH interception cannot see.
package shim

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticraptor/agent-leash/internal/guard"
	"github.com/agenticraptor/agent-leash/internal/policy"
	"github.com/agenticraptor/agent-leash/internal/session"
)

// Environment variables the supervisor sets for shims and guard-exec.
const (
	EnvBin       = "AGENT_LEASH_BIN"       // absolute path to the agent-leash binary
	EnvShimDir   = "AGENT_LEASH_SHIM_DIR"  // directory holding the shims (excluded from PATH lookups)
	EnvSession   = "AGENT_LEASH_SESSION"   // session directory for the events log
	EnvWorkspace = "AGENT_LEASH_WORKSPACE" // absolute workspace root
	EnvActive    = "AGENT_LEASH_ACTIVE"    // set to "1" inside a leashed session
)

// BlockedExitCode is returned by guard-exec when it refuses to run a command.
const BlockedExitCode = 113

// SafeName reports whether a guarded command name is a plain binary name (no
// path separators, no traversal, no shell metacharacters). Names come from a
// policy file that may live in an untrusted repository, so anything that could
// escape the shim directory or inject into a shim script is rejected.
func SafeName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`+"\x00") {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		// Allow only conservative, portable executable-name characters.
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.' || r == '+':
		default:
			return false
		}
	}
	return true
}

// Install writes a shim for each safe command name into dir, each delegating to
// `binPath guard-exec`. dir is created if necessary. Unsafe names are skipped
// (a hostile policy cannot make Install write outside dir).
func Install(binPath, dir string, names []string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range names {
		if !SafeName(name) {
			continue
		}
		file, content := script(name, binPath)
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o755); err != nil { //nolint:gosec // shims must be executable
			return fmt.Errorf("write shim %s: %w", name, err)
		}
	}
	return nil
}

// RunGuardExec is the body of `agent-leash guard-exec`. It evaluates an
// intercepted command against the active policy, records the decision to the
// session log, and either executes the real binary or blocks it. It returns a
// process exit code (on Unix the allow path replaces the process and does not
// return).
func RunGuardExec(name string, args []string, stderr io.Writer) int {
	cwd, _ := os.Getwd()
	pol, _, err := policy.LoadAuto(os.Getenv(policy.EnvVar), cwd)
	if err != nil {
		pol = policy.Default()
	}
	ws := os.Getenv(EnvWorkspace)
	if ws == "" {
		ws = cwd
	}

	argv := append([]string{name}, args...)
	d := guard.New(pol, ws).CheckArgv(argv)
	action := pol.OnViolation.Action

	_ = session.Append(os.Getenv(EnvSession), session.Event{
		Type:     session.TypeCommand,
		Command:  strings.Join(argv, " "),
		Allow:    d.Allow,
		Category: string(d.Category),
		Reason:   d.Reason,
		Rule:     d.Rule,
		Action:   action,
	})

	// "warn" downgrades a denial to a logged warning and lets the command run.
	if d.Allow || action == policy.ActionWarn {
		if !d.Allow {
			fmt.Fprintf(stderr, "⚠ agent-leash: %s — allowed (action = warn)\n", d.Reason)
		}
		return execReal(name, argv, stderr)
	}

	fmt.Fprintf(stderr, "\n⛔ agent-leash blocked a command\n   %s\n   $ %s\n\n",
		d.Reason, strings.Join(argv, " "))
	return BlockedExitCode
}

func execReal(name string, argv []string, stderr io.Writer) int {
	real, err := LookPathExcluding(name, os.Getenv(EnvShimDir))
	if err != nil {
		fmt.Fprintf(stderr, "agent-leash: cannot locate the real %q: %v\n", name, err)
		return 127
	}
	if err := ExecReal(real, argv); err != nil { // returns only on failure
		fmt.Fprintf(stderr, "agent-leash: failed to exec %q: %v\n", real, err)
		return 126
	}
	return 0
}

// LookPathExcluding resolves a command name to a real executable, ignoring the
// shim directory so a shim never resolves to itself. If name already contains a
// path separator it is returned via the normal lookup.
func LookPathExcluding(name, excludeDir string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		return exec.LookPath(name)
	}
	orig := os.Getenv("PATH")
	excl := ""
	if excludeDir != "" {
		excl, _ = filepath.Abs(excludeDir)
	}
	var kept []string
	for _, dir := range filepath.SplitList(orig) {
		if dir == "" {
			continue
		}
		if abs, _ := filepath.Abs(dir); excl != "" && abs == excl {
			continue
		}
		kept = append(kept, dir)
	}
	// Reuse exec.LookPath (which knows about PATHEXT on Windows) against the
	// filtered PATH, then restore the original.
	_ = os.Setenv("PATH", strings.Join(kept, string(os.PathListSeparator)))
	defer os.Setenv("PATH", orig) //nolint:errcheck // restoring our own value
	return exec.LookPath(name)
}
