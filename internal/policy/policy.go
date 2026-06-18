// Package policy defines the rules agent-leash enforces around an agent session
// and loads them from an optional TOML file. A policy describes what an agent is
// allowed to do this session — how many files it may change, how many new
// dependencies it may add, how long it may run, how much it may spend, whether
// it may touch the network, and which commands are forbidden outright.
//
// agent-leash works with no config at all: Default() returns a protective but
// non-paranoid policy. A project-local .agent-leash.toml (or the XDG file) only
// exists to tighten or loosen those defaults.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config is the full, on-disk policy.
type Config struct {
	Limits      Limits      `toml:"limits"`
	Network     Network     `toml:"network"`
	Filesystem  Filesystem  `toml:"filesystem"`
	Commands    Commands    `toml:"commands"`
	OnViolation OnViolation `toml:"on_violation"`
}

// Limits are the countable budgets for a session. A zero value means "no limit"
// for every field except where noted, so an empty [limits] table is permissive.
type Limits struct {
	MaxFilesChanged int      `toml:"max_files_changed"` // distinct files created/modified; 0 = unlimited
	MaxNewDeps      int      `toml:"max_new_deps"`      // dependencies added across manifests; 0 = unlimited
	MaxCommands     int      `toml:"max_commands"`      // guarded commands executed; 0 = unlimited
	MaxDuration     Duration `toml:"max_duration"`      // wall-clock budget; 0 = unlimited
	MaxCostUSD      float64  `toml:"max_cost_usd"`      // spend budget, enforced when usage is reported; 0 = unlimited
}

// Network controls whether the agent may reach the network.
type Network struct {
	// Allowed, when false, denies known network commands (curl, wget, nc, …) and,
	// under `agent-leash run --harden` on Linux, drops the child into a private
	// network namespace with no connectivity.
	Allowed bool `toml:"allowed"`
}

// Filesystem confines where the agent may write.
type Filesystem struct {
	Workspace    string   `toml:"workspace"`     // root the agent may modify (default ".")
	AllowOutside bool     `toml:"allow_outside"` // permit writes outside the workspace
	Protect      []string `toml:"protect"`       // paths that are always off-limits (e.g. ~/.ssh)
}

// Commands controls which command lines are allowed to run.
type Commands struct {
	// Deny is a list of glob-style patterns matched against each command segment
	// of a (possibly chained) command line. A match is a hard violation.
	Deny []string `toml:"deny"`
	// Allow, when non-empty, switches to allowlist mode: only command words in
	// this list may run; everything else is denied.
	Allow []string `toml:"allow"`
	// Guard is the set of binary names that get PATH shims under `run`, so their
	// invocations are inspected before they execute. Other binaries run normally.
	Guard []string `toml:"guard"`
}

// OnViolation decides what happens when a limit is crossed.
type OnViolation struct {
	Action    string   `toml:"action"`     // "stop" (default), "warn", or "ask"
	KillGrace Duration `toml:"kill_grace"` // graceful window before a forceful kill
}

// Action values for OnViolation.Action.
const (
	ActionStop = "stop"
	ActionWarn = "warn"
	ActionAsk  = "ask"
)

// Default returns the policy used when no file is present: protective about
// genuinely destructive actions and secret directories, generous about ordinary
// work, with the network left on so package installs keep working.
func Default() Config {
	return Config{
		Limits: Limits{
			MaxFilesChanged: 50,
			MaxNewDeps:      5,
			MaxCommands:     0,
			MaxDuration:     Duration{30 * time.Minute},
			MaxCostUSD:      0,
		},
		Network: Network{Allowed: true},
		Filesystem: Filesystem{
			Workspace:    ".",
			AllowOutside: false,
			Protect:      defaultProtect(),
		},
		Commands: Commands{
			Deny:  DefaultDeny(),
			Allow: nil,
			Guard: DefaultGuard(),
		},
		OnViolation: OnViolation{
			Action:    ActionStop,
			KillGrace: Duration{5 * time.Second},
		},
	}
}

// DefaultDeny is the built-in deny list: patterns that should almost never run
// unattended. Patterns are matched case-sensitively against normalized command
// lines with wildcard semantics, where `*` matches any run of characters
// (including spaces and `/`) and `?` matches exactly one.
func DefaultDeny() []string {
	return []string{
		"rm -rf /*",             // wiping the filesystem root
		"rm -rf ~*",             // wiping the home directory
		"rm -rf /",              //
		"rm -fr /*",             //
		"* --no-preserve-root*", // explicit root-wipe opt-in
		"dd of=/dev/*",          // overwriting a block device
		"mkfs*",                 // formatting a filesystem
		"shred *",               // unrecoverable deletion
		"git push --force*",     // history rewrite on a remote
		"git push -f*",          //
		"git reset --hard*",     // discarding uncommitted work
		"git clean -fd*",        // deleting untracked files
		"* | sh",                // piping a download straight into a shell
		"* | bash",              //
		"curl * | *sh",          //
		"wget * | *sh",          //
		"chmod -R 777 *",        // world-writable trees
		":(){ :|:& };:",         // fork bomb
		"sudo *",                // privilege escalation
		"doas *",                //
	}
}

// DefaultGuard lists the binaries that are worth inspecting before they run. It
// deliberately includes both always-dangerous tools (rm, dd) and ordinary tools
// with destructive modes (git, npm), so a bad invocation is caught while normal
// use passes straight through.
func DefaultGuard() []string {
	return []string{
		"rm", "rmdir", "dd", "mkfs", "shred", "srm",
		"chmod", "chown", "mv",
		"curl", "wget", "scp", "ssh", "sftp", "nc", "ncat", "telnet",
		"sudo", "doas",
		"git", "npm", "pnpm", "yarn", "pip", "pip3", "gem", "cargo", "brew",
	}
}

func defaultProtect() []string {
	return []string{
		"~/.ssh", "~/.aws", "~/.config/gh", "~/.gnupg",
		"~/.kube", "~/.docker/config.json", "~/.npmrc", "~/.netrc",
	}
}

// Validate checks the policy for internal consistency and normalizes a few
// fields. It returns an error describing the first problem found.
func (c *Config) Validate() error {
	switch c.OnViolation.Action {
	case "", ActionStop:
		c.OnViolation.Action = ActionStop
	case ActionWarn, ActionAsk:
		// ok
	default:
		return fmt.Errorf("on_violation.action: unknown action %q (want stop, warn, or ask)", c.OnViolation.Action)
	}
	if c.Filesystem.Workspace == "" {
		c.Filesystem.Workspace = "."
	}
	if c.OnViolation.KillGrace.Duration < 0 {
		return fmt.Errorf("on_violation.kill_grace: must not be negative")
	}
	if c.Limits.MaxCostUSD < 0 {
		return fmt.Errorf("limits.max_cost_usd: must not be negative")
	}
	for _, p := range c.Commands.Deny {
		if _, err := filepathMatchProbe(p); err != nil {
			return fmt.Errorf("commands.deny: %q is not a valid pattern: %w", p, err)
		}
	}
	// Guard names become shim filenames, so they must be plain binary names —
	// a policy file may come from an untrusted repository.
	for _, name := range c.Commands.Guard {
		if !isPlainCommandName(name) {
			return fmt.Errorf("commands.guard: %q is not a valid command name (no paths or special characters)", name)
		}
	}
	return nil
}

// isPlainCommandName reports whether name is a bare executable name with no path
// separators, traversal, or unusual characters.
func isPlainCommandName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.' || r == '+':
		default:
			return false
		}
	}
	return true
}

// WorkspaceRoot returns the absolute, cleaned workspace directory, resolving a
// relative workspace against base (typically the current directory).
func (c *Config) WorkspaceRoot(base string) (string, error) {
	ws := ExpandPath(c.Filesystem.Workspace)
	if !filepath.IsAbs(ws) {
		ws = filepath.Join(base, ws)
	}
	abs, err := filepath.Abs(ws)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// ProtectedPaths returns the protected paths with ~ and environment variables
// expanded to absolute paths.
func (c *Config) ProtectedPaths() []string {
	out := make([]string, 0, len(c.Filesystem.Protect))
	for _, p := range c.Filesystem.Protect {
		if e := ExpandPath(p); e != "" {
			out = append(out, filepath.Clean(e))
		}
	}
	return out
}

// ExpandPath expands a leading ~ and any $VAR references to an absolute-ish path.
func ExpandPath(p string) string {
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return os.ExpandEnv(p)
}

func filepathMatchProbe(pattern string) (bool, error) {
	return filepath.Match(pattern, "")
}
