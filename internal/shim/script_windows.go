//go:build windows

package shim

import "strings"

// script returns the filename and contents of a Windows batch shim that
// forwards the intercepted command to `agent-leash guard-exec`.
func script(name, binPath string) (string, string) {
	var b strings.Builder
	b.WriteString("@echo off\r\n")
	b.WriteString("rem agent-leash shim — generated, safe to delete.\r\n")
	b.WriteString("\"")
	b.WriteString(binPath)
	b.WriteString("\" guard-exec --name \"")
	b.WriteString(name)
	b.WriteString("\" -- %*\r\n")
	return name + ".bat", b.String()
}

// shellQuote is unused on Windows but kept for parity with the Unix build.
func shellQuote(s string) string { return s }
