//go:build unix

package server

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lock takes an exclusive, non-blocking flock on path.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another piggery serve holds %s", path)
		}
		return nil, err
	}
	return func() { f.Close() }, nil
}
