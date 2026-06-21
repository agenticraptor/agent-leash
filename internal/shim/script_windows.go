//go:build windows

package shim

import "strings"

// script returns the filename and contents of a Windows batch shim that
// forwards the intercepted command to `agent-leash guard-exec`.
func script(name, binPath string) (string, string) {
	body := strings.Join([]string{
		"@echo off",
		"rem agent-leash shim — generated, safe to delete.",
		"\"" + binPath + "\" guard-exec --name \"" + name + "\" -- %*",
		"",
	}, "\r\n")
	return name + ".bat", body
}

// shellQuote is unused on Windows but kept for parity with the Unix build.
func shellQuote(s string) string { return s }
