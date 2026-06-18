package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ConfigName is the project-local policy file agent-leash looks for, walking up
// from the working directory.
const ConfigName = ".agent-leash.toml"

// EnvVar lets a user (or the supervisor) point at an explicit policy file.
const EnvVar = "AGENT_LEASH_POLICY"

// XDGPath returns the location of the user-level policy file, honoring
// XDG_CONFIG_HOME.
func XDGPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "agent-leash", "policy.toml"), nil
}

// Resolve finds the policy file to use, in priority order:
//
//  1. explicit (a --policy flag value), if non-empty;
//  2. the AGENT_LEASH_POLICY environment variable, if set;
//  3. the nearest .agent-leash.toml at or above startDir;
//  4. the XDG user policy, if it exists.
//
// It returns the path and whether a file was actually found. When nothing is
// found it returns ("", false) and callers should fall back to Default().
func Resolve(explicit, startDir string) (string, bool) {
	if explicit != "" {
		return explicit, true
	}
	if env := os.Getenv(EnvVar); env != "" {
		return env, true
	}
	if p, ok := findUp(startDir, ConfigName); ok {
		return p, true
	}
	if p, err := XDGPath(); err == nil {
		if _, statErr := os.Stat(p); statErr == nil {
			return p, true
		}
	}
	return "", false
}

// findUp walks from dir toward the filesystem root looking for name.
func findUp(dir, name string) (string, bool) {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(abs, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}

// Load reads a policy file over the built-in defaults and validates the result.
// Unknown keys are tolerated so a newer file does not break an older binary.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the user's own config discovery
	if err != nil {
		return cfg, err
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// LoadAuto resolves and loads the active policy. When no file is found it
// returns the validated defaults and an empty path.
func LoadAuto(explicit, startDir string) (Config, string, error) {
	path, ok := Resolve(explicit, startDir)
	if !ok {
		cfg := Default()
		if err := cfg.Validate(); err != nil {
			return cfg, "", err
		}
		return cfg, "", nil
	}
	cfg, err := Load(path)
	return cfg, path, err
}

// Init writes a documented starter policy into dir (as .agent-leash.toml) and
// returns its path. It never overwrites an existing file.
func Init(dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	return InitFile(filepath.Join(dir, ConfigName))
}

// InitFile writes the documented starter policy to an explicit path, creating
// parent directories as needed. It never overwrites an existing file.
func InitFile(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		return path, errors.New("a policy file already exists here")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(starter), 0o644); err != nil { //nolint:gosec // a policy file is not a secret
		return "", err
	}
	return path, nil
}

// Starter returns the documented starter policy text.
func Starter() string { return starter }

const starter = `# agent-leash policy — every field is optional and overrides a built-in default.
# This file caps what an agent may do in a session. Docs:
# https://github.com/agenticraptor/agent-leash/blob/main/docs/policy.md

[limits]
# Hard ceilings for the session. 0 (or an empty duration) means "no limit".
max_files_changed = 50    # distinct files the agent may create or modify
max_new_deps      = 5      # dependencies it may add across all manifests
max_commands      = 0      # guarded commands it may run (0 = unlimited)
max_duration      = "30m"  # wall-clock budget, e.g. "20m", "1h30m"
max_cost_usd      = 0       # spend budget; enforced when usage is reported (see docs)

[network]
# false denies known network commands (curl, wget, nc, …). With
# 'agent-leash run --harden' on Linux it also drops the agent into a private
# network namespace with no connectivity.
allowed = true

[filesystem]
workspace     = "."   # the only tree the agent may modify
allow_outside = false # block file changes outside the workspace
# Paths that are always off-limits, even inside the workspace:
protect = ["~/.ssh", "~/.aws", "~/.config/gh", "~/.gnupg", "~/.npmrc", "~/.netrc"]

[commands]
# deny: glob patterns ('*' and '?') matched against each command in a chained
# line. A match hard-stops the session. These extend the built-in deny list.
deny = [
  "rm -rf /*",
  "git push --force*",
  "* | sh",
  "* | bash",
]
# allow: if non-empty, switches to allowlist mode — only these command words may
# run and everything else is denied. Leave empty for deny-list mode.
allow = []
# guard: binaries that get inspected before they run under 'run'. Override only
# if you need to add or remove tools from the built-in set.
# guard = ["rm", "dd", "git", "npm", "curl", "wget", "sudo"]

[on_violation]
action     = "stop"  # stop (kill the agent), warn (log and continue), or ask
kill_grace = "5s"     # graceful window before a forceful kill
`
