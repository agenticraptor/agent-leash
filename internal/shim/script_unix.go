//go:build !windows

package shim

import "strings"

// script returns the filename and contents of a POSIX shell shim that forwards
// the intercepted command to `agent-leash guard-exec`.
func script(name, binPath string) (string, string) {
	body := strings.Join([]string{
		"#!/bin/sh",
		"# agent-leash shim — generated, safe to delete. Intercepts `" + name + "` for policy enforcement.",
		"exec " + shellQuote(binPath) + " guard-exec --name " + shellQuote(name) + " -- \"$@\"",
		"",
	}, "\n")
	return name, body
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
