//go:build !linux

package sandbox

// Wrap is a no-op on non-Linux platforms; OS-level network isolation is only
// implemented for Linux today. Command-level network guards still apply.
func Wrap(argv []string, opt Options) Plan {
	note := ""
	if opt.Harden {
		note = "OS-level hardening is currently Linux-only; relying on command-level guards"
	}
	return Plan{Argv: argv, Note: note}
}
