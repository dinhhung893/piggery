//go:build !windows

package local

import "os"

// symlinkOrCopy creates a symlink (unix behavior — cheap and always tracks the source).
func symlinkOrCopy(src, dst string) error {
	return os.Symlink(src, dst)
}
