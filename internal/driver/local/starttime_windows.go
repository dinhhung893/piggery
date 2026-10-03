//go:build windows

package local

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcessStartTime is the OS start time of pid in unix ms, or 0 when unknown. Recovery
// compares it with the live process before killing anything, so a reused pid
// is never mistaken for the worker; a Claude session's host key uses it the same way.
// Windows implementation: OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION) + GetProcessTimes.
func ProcessStartTime(pid int) int64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	var ct, et, kt, ut windows.Filetime
	if err := windows.GetProcessTimes(h, &ct, &et, &kt, &ut); err != nil {
		return 0
	}
	return ct.Nanoseconds() / int64(1000000) // nanoseconds -> milliseconds
}

// ParentPID is pid's parent, or 0 when unknown (the process is gone).
// Windows implementation: toolhelp32 snapshot lookup.
func ParentPID(pid int) int {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for ok := windows.Process32First(snap, &e); ok == nil; ok = windows.Process32Next(snap, &e) {
		if int(e.ProcessID) == pid {
			return int(e.ParentProcessID)
		}
	}
	return 0
}
