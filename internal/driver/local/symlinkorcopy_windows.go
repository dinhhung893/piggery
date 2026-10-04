//go:build windows

package local

import (
	"os"
)

// symlinkOrCopy copies the file on Windows (os.Symlink requires elevated privileges).
// A copy is semantically equivalent: the worker gets a snapshot of the config at spawn
// time. Changes to the source after spawn are not tracked (on unix, the symlink tracks
// them), but for a short-lived worker this is acceptable.
func symlinkOrCopy(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
