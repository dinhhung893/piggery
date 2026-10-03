//go:build unix

package server

import (
	"os/exec"
	"syscall"
)

// setupHookKill gives the hook a process group of its own, so the timeout kill reaches whatever
// the hook started too, and makes exec's Cancel kill that group.
func setupHookKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
