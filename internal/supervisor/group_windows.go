//go:build windows

package supervisor

import (
	"os/exec"
	"strconv"
	"syscall"
)

// createNewProcessGroup is the Windows flag that starts the child in a new
// process group, so it can be signaled independently of agent-leash.
const createNewProcessGroup = 0x00000200

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

// killGroup terminates the child and its descendants with taskkill /T. Windows
// has no graceful group signal equivalent to SIGTERM, so both modes force-kill
// the tree.
func killGroup(cmd *exec.Cmd, _ bool) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run() //nolint:gosec // fixed args
}
