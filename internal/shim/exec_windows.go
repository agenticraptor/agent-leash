//go:build windows

package shim

import (
	"os"
	"os/exec"
)

// ExecReal runs the real binary to completion (Windows has no exec-replace) and
// exits with its status, so it does not return on success.
func ExecReal(path string, argv []string) error {
	cmd := exec.Command(path, argv[1:]...) //nolint:gosec // path resolved from PATH, argv from the agent
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err == nil {
		os.Exit(0)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		os.Exit(ee.ExitCode())
	}
	return err
}
