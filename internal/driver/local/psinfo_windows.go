//go:build windows

package local

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// psInfo reads the OS start time (unix ms) and command of pid on Windows.
// alive=false when the process does not exist; err when it cannot be read.
func psInfo(ctx context.Context, pid int) (start int64, command string, alive bool, err error) {
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, "", false, nil // no such process
	}
	defer windows.CloseHandle(h)
	var ct, et, kt, ut windows.Filetime
	if err := windows.GetProcessTimes(h, &ct, &et, &kt, &ut); err != nil {
		return 0, "", false, fmt.Errorf("read process times: %w", err)
	}
	// Get the image name for sameProgram comparison
	snap, err2 := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err2 != nil {
		return 0, "", false, fmt.Errorf("read process table: %w", err2)
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	command = ""
	for ok := windows.Process32First(snap, &e); ok == nil; ok = windows.Process32Next(snap, &e) {
		if int(e.ProcessID) == pid {
			command = windows.UTF16ToString(e.ExeFile[:])
			break
		}
	}
	return ct.Nanoseconds() / int64(1000000), command, true, nil
}
