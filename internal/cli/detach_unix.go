//go:build unix

package cli

import (
	"os/exec"
	"syscall"
)

// detachDaemon starts the daemon in a session of its own, so it outlives this client.
func detachDaemon(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
