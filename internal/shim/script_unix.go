//go:build !windows

package shim

import "strings"

// script returns the filename and contents of a POSIX shell shim that forwards
// the intercepted command to `agent-leash guard-exec`.
func script(name, binPath string) (string, string) {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# agent-leash shim — generated, safe to delete. Intercepts `")
	b.WriteString(name)
	b.WriteString("` for policy enforcement.\n")
	b.WriteString("exec ")
	b.WriteString(shellQuote(binPath))
	b.WriteString(" guard-exec --name ")
	b.WriteString(shellQuote(name))
	b.WriteString(" -- \"$@\"\n")
	return name, b.String()
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
