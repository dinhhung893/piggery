//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

// detachDaemon starts the daemon detached from this client's console (CREATE_NO_WINDOW, so no
// console window pops up; the log files carry stdout/stderr). Windows has no sessions: the child
// outlives the parent by default, so no extra step is needed.
func detachDaemon(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
