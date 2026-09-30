package local

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// A harness runs its tools in child processes, often each in a process group of its own (pi's
// bash tool does), so signalling the worker's group misses them; and once the harness dies its
// children reparent to init and can no longer be found. So the tree is read first, from the
// worker's pid down, and every member is signalled: its group when the group's leader is in the
// tree too, otherwise the pid alone.

// proc is one row of the process table. start is `ps lstart` verbatim: a pid is the same process
// only while its start is too.
type proc struct {
	pid, ppid, pgid int
	start           string
}

// processTable is every process (`ps -A`, on macOS and Linux); nil when it cannot be read.
func processTable() []proc {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,pgid=,lstart=").Output()
	if err != nil {
		return nil
	}
	var rows []proc
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 8 {
			continue
		}
		pid, e1 := strconv.Atoi(f[0])
		ppid, e2 := strconv.Atoi(f[1])
		pgid, e3 := strconv.Atoi(f[2])
		if e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		rows = append(rows, proc{pid, ppid, pgid, strings.Join(f[3:], " ")})
	}
	return rows
}

// treeBelow is the descendants of root (root itself not included; it is signalled by its group),
// plus every process of known that is still the same one, with their descendants. known is an
// earlier treeBelow: it finds what has been reparented since. root is a seed only when it is alive
// (a reaped pid may belong to someone else now).
func treeBelow(root int, rootAlive bool, known []proc) []proc {
	rows := processTable()
	byPid := make(map[int]proc, len(rows))
	kids := map[int][]proc{}
	for _, r := range rows {
		byPid[r.pid] = r
		kids[r.ppid] = append(kids[r.ppid], r)
	}
	var out []proc
	seen := map[int]bool{root: true}
	var walk func(pid int)
	walk = func(pid int) {
		for _, k := range kids[pid] {
			if !seen[k.pid] {
				seen[k.pid] = true
				out = append(out, k)
				walk(k.pid)
			}
		}
	}
	if rootAlive {
		walk(root)
	}
	for _, k := range known {
		if r, ok := byPid[k.pid]; ok && r.start == k.start && !seen[k.pid] {
			seen[k.pid] = true
			out = append(out, r)
			walk(k.pid)
		}
	}
	return out
}

// signalTree sends sig to the worker's group pgid (when group: the leader has not been reaped, or
// was alive when this stop began) and to each member of tree.
func (d *Driver) signalTree(pgid int, group bool, tree []proc, sig syscall.Signal) {
	if group {
		d.kill(-pgid, sig)
	}
	own := syscall.Getpgrp()
	leader := map[int]bool{}
	for _, p := range tree {
		leader[p.pid] = true
	}
	sent := map[int]bool{pgid: group}
	for _, p := range tree {
		if p.pgid > 1 && p.pgid != own && !sent[p.pgid] && leader[p.pgid] {
			sent[p.pgid] = true
			d.kill(-p.pgid, sig)
		}
	}
	for _, p := range tree {
		if !sent[p.pgid] && p.pid > 1 {
			d.kill(p.pid, sig)
		}
	}
}
