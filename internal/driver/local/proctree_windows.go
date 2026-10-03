//go:build windows

package local

import (
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A worker's job object (JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE) stands in for the unix process
// group: TerminateJobObject kills the worker and everything it has spawned at once, including
// children that appeared after the last process-table read. jobs maps the worker's pgid (its
// pid: Windows has no process groups) to the job. Handles stay open for the life of the
// process — one per worker ever run — so a late kill still reaches the tree; the job kills
// whatever is left if this process exits first.
var (
	jobsMu sync.Mutex
	jobs   = map[int]windows.Handle{} // pgid -> job object
)

// processTable is every process, from the toolhelp snapshot; nil when it cannot be read.
// pgid is the pid (Windows has no process groups).
func processTable() []proc {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var rows []proc
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for ok := windows.Process32First(snap, &e); ok == nil; ok = windows.Process32Next(snap, &e) {
		pid := int(e.ProcessID)
		rows = append(rows, proc{pid: pid, ppid: int(e.ParentProcessID), pgid: pid, start: procStart(pid)})
	}
	return rows
}

// procStart is the process's creation time as an identity string, "" when unknown (system
// processes cannot be opened).
func procStart(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var ct, et, kt, ut windows.Filetime
	if err := windows.GetProcessTimes(h, &ct, &et, &kt, &ut); err != nil {
		return ""
	}
	return strconv.FormatInt(ct.Nanoseconds(), 10)
}

// signalTree sends sig to the worker's job (when group and the leader is not reaped: the job
// terminates the whole tree at once) and to each member of tree that is not in it.
func (d *Driver) signalTree(pgid int, group bool, tree []proc, sig syscall.Signal) {
	if sig != syscall.SIGKILL {
		return // Windows has no deliverable SIGTERM; the graceful step is the stdin close in Stop.
	}
	if group {
		d.kill(-pgid, sig)
	}
	for _, p := range tree {
		d.kill(p.pid, sig)
	}
}

// newKill is the kill used by the runner: SIGKILL terminates (the job for -pgid, the process
// otherwise); SIGTERM cannot be delivered on Windows, so it is a no-op.
func newKill() func(pid int, sig syscall.Signal) error {
	return func(pid int, sig syscall.Signal) error {
		if sig != syscall.SIGKILL {
			return nil
		}
		if pid < 0 {
			jobsMu.Lock()
			job := jobs[-pid]
			jobsMu.Unlock()
			if job == 0 {
				return syscall.EINVAL
			}
			return windows.TerminateJobObject(job, 1)
		}
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
		if err != nil {
			return err
		}
		defer windows.CloseHandle(h)
		return windows.TerminateProcess(h, 1)
	}
}

// setPgid is a no-op on Windows: the job object assigned in attachJob covers the tree.
func setPgid(_ *exec.Cmd) {}

// attachJob creates the worker's kill-tree job object and assigns its process to it; a failure
// leaves the worker unassigned (signalTree still kills the table-read tree by pid).
func attachJob(p *os.Process, w *worker) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return
	}
	if err := windows.AssignProcessToJobObject(job, procHandle(p)); err != nil {
		windows.CloseHandle(job)
		return
	}
	jobsMu.Lock()
	jobs[w.pgid] = job
	jobsMu.Unlock()
	w.job = uintptr(job)
}
// procHandle is the worker's real handle from its pid, for AssignProcessToJobObject. Go's
// os.Process keeps only a pseudo handle (current-process-relative), which would assign the
// wrong process. PROCESS_SET_QUOTA | PROCESS_TERMINATE are the rights AssignProcessToJobObject
// needs; 0 on failure and attachJob gives up.
func procHandle(p *os.Process) windows.Handle {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return 0
	}
	return h
}
