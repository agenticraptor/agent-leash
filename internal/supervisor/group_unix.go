//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so the whole tree can
// be signalled at once.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup signals the child's entire process group, gracefully (SIGTERM) or
// forcefully (SIGKILL).
func killGroup(cmd *exec.Cmd, force bool) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, sig) // negative pid targets the whole group
		return
	}
	_ = cmd.Process.Signal(sig)
}
