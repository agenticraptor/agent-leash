// Package sandbox optionally strengthens a leashed session with operating-system
// isolation on top of the command-level guards. Today it can drop a child into a
// private network namespace on Linux (so "network disabled" is enforced by the
// kernel, not just by intercepting known network tools). On other platforms, or
// when the required tools are unavailable, it is a no-op and says so, leaving the
// command-level guards in place.
package sandbox

// Options requests OS-level hardening for a session.
type Options struct {
	Harden         bool // the user asked for --harden
	NetworkAllowed bool // whether the policy permits network access
}

// Plan is the (possibly wrapped) command to launch plus a description of what,
// if any, OS-level isolation was applied.
type Plan struct {
	Argv     []string // the command to actually exec
	Note     string   // human-readable description of hardening (or why none)
	Hardened bool     // true if real OS isolation was applied
}
