//go:build windows

package local

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// symlinkOrCopy handles the src→dst link on Windows.
//   - Directories: create a junction/symbolic link with
//     SYMBOLIC_LINK_FLAG_DIRECTORY|SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE
//     (no admin needed on Win10 1703+ with Developer Mode, or always works for
//     same-volume junctions via CreateSymbolicLinkW with the junction flag).
//   - Files: copy content (a snapshot at spawn time).
func symlinkOrCopy(src, dst string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("lstat %s: %w", src, err)
	}
	if fi.IsDir() {
		// Remove dst if it already exists
		os.RemoveAll(dst)
		// CreateSymbolicLinkW with DIRECTORY flag creates a junction-like link
		// that works without admin for same-volume targets.
		// The flags: SYMBOLIC_LINK_FLAG_DIRECTORY (0x1) | SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE (0x2)
		// On Windows 10 1703+, ALLOW_UNPRIVILEGED_CREATE works when Developer Mode is on.
		// Without Developer Mode, it falls back to requiring admin.
		// For maximum compatibility, use a hard link for files and exec mklink /J for dirs.
		// But os.Symlink actually handles the DIRECTORY flag internally on Windows.
		// The issue is only the privilege — let's try os.Symlink first, fall back to copy.
		err = os.Symlink(src, dst)
		if err == nil {
			return nil
		}
		// Fallback: junction via CreateSymbolicLink with DIRECTORY | ALLOW_UNPRIVILEGED_CREATE
		dstPtr, err1 := windows.UTF16PtrFromString(dst)
		srcPtr, err2 := windows.UTF16PtrFromString(src)
		if err1 == nil && err2 == nil {
			err = windows.CreateSymbolicLink(dstPtr, srcPtr, windows.SYMBOLIC_LINK_FLAG_DIRECTORY|0x2)
		}
		if err == nil {
			return nil
		}
		// Last resort: recursive copy (slow but always works)
		return copyDir(src, dst)
	}
	// Regular file: copy content
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	return os.WriteFile(dst, data, 0o600)
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := src + "\\" + e.Name()
		d := dst + "\\" + e.Name()
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(s)
			if err != nil {
				return err
			}
			if err := os.WriteFile(d, data, 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}
