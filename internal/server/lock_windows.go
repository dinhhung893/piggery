//go:build windows

package server

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// lock takes an exclusive, non-blocking byte-range lock on path (LockFileEx in place of unix
// flock); unlocking it releases the lock and closes the file.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	ol := new(windows.Overlapped)
	h := windows.Handle(f.Fd())
	if err := windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, ol); err != nil {
		f.Close()
		return nil, fmt.Errorf("another piggery serve holds %s", path)
	}
	return func() {
		windows.UnlockFileEx(h, 0, 1, 0, ol)
		f.Close()
	}, nil
}
