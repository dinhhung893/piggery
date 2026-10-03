//go:build !windows

package server

import (
	"os"
)

// hookRunnable reports whether the file at path is a runnable notify hook.
// Unix: the file must be a regular file with at least one exec bit set.
func hookRunnable(fi os.FileInfo) bool {
	return fi.Mode().IsRegular() && fi.Mode()&0o111 != 0
}

// hookRunnableExt reports whether the file extension could be a runnable hook on this platform
// (used for the setup warning when no hooks are found).
func hookRunnableExt(name string) bool { return true }
