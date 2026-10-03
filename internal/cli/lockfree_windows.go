//go:build windows

package cli

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockFree reports whether no daemon holds the singleton lock at path (same LockFileEx byte
// range the daemon's lock takes in internal/server).
func lockFree(path string) (bool, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol); err != nil {
		return false, nil
	}
	windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
	return true, nil
}
