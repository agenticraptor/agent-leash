//go:build !windows

package shim

import (
	"os"
	"syscall"
)

// ExecReal replaces the current process with the real binary, preserving argv
// and the environment. On success it does not return.
func ExecReal(path string, argv []string) error {
	return syscall.Exec(path, argv, os.Environ())
}
