//go:build windows

package server

import (
	"os/exec"
)

// setupHookKill arms exec's Cancel on windows: Process.Kill terminates the hook itself. Windows
// has no process groups, so children the hook started survive it; hooks are single short-lived
// processes and WaitDelay reaps the run either way.
func setupHookKill(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Kill() }
}
