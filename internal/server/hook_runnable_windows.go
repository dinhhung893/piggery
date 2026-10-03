//go:build windows

package server

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// hookRunnable reports whether the file at path is a runnable notify hook.
// Windows: exec bits are not meaningful; accept known executable extensions.
func hookRunnable(fi os.FileInfo) bool {
	if !fi.Mode().IsRegular() {
		return false
	}
	ext := strings.ToLower(filepath.Ext(fi.Name()))
	switch ext {
	case ".exe", ".cmd", ".bat", ".ps1", ".sh":
		return true
	}
	return false
}

// hookRunnableExt reports whether the file extension could be a runnable hook on this platform.
func hookRunnableExt(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".exe", ".cmd", ".bat", ".ps1", ".sh":
		return true
	}
	return false
}
