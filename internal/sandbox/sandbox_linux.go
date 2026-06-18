//go:build linux

package sandbox

import "os/exec"

// Wrap drops the command into an unprivileged user+network namespace via
// `unshare` when hardening is requested and the policy forbids the network, so
// the agent has no network connectivity at the kernel level. If unshare is not
// available it falls back to command-level guards and explains the gap.
func Wrap(argv []string, opt Options) Plan {
	if !opt.Harden {
		return Plan{Argv: argv}
	}
	if opt.NetworkAllowed {
		return Plan{Argv: argv, Note: "no OS network isolation applied (policy allows the network)"}
	}
	path, err := exec.LookPath("unshare")
	if err != nil {
		return Plan{Argv: argv, Note: "`unshare` not found; network is blocked at the command level only"}
	}
	wrapped := append([]string{path, "--user", "--map-root-user", "--net", "--"}, argv...)
	return Plan{
		Argv:     wrapped,
		Note:     "Linux network namespace with no connectivity (via unshare)",
		Hardened: true,
	}
}
