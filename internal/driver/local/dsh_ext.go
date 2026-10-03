package local

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	dshext "github.com/sting8k/piggery/extensions/dsh"
)

// piggery's dsh plugin installed from the binary (managedext.go): the tree of dshext.Tree under
// <piggery dir>/dsh, one copy for the sessions the Human opens (`piggery setup dsh` adds a row for
// its entry to dsh's home patch) and for workers (a --patch overlay names the same entry).

// dshManaged is the plugin as a managed copy: dsh/index.mjs carries the marker.
var dshManaged = managedExt{
	setup:      "dsh",
	markerFile: dshext.Entry,
	hint:       "move it away",
	files:      dshext.Tree,
}

// DshExtDir is where the plugin is installed (`piggery setup dsh`, and a worker's start).
func DshExtDir(dir string) string { return PluginDir(dir, "dsh") }

// DshEntry is the file dsh loads as the plugin.
func DshEntry(dir string) string {
	return filepath.Join(DshExtDir(dir), filepath.FromSlash(dshext.Entry))
}

// DshExtVersion is the integration version of the managed copy at ext; managed is false when ext
// holds none.
func DshExtVersion(ext string) (version int, managed bool) { return dshManaged.Version(ext) }

// DshExtCurrent: the managed copy at ext holds exactly what this binary would write.
func DshExtCurrent(ext string) bool { return dshManaged.Current(ext) }

// InstallDshExt writes the plugin into ext, in place of a managed copy. Anything else there is
// refused. It reports whether it wrote.
func InstallDshExt(ext string) (bool, error) { return dshManaged.Install(ext) }

// UpdateDshExt (daemon start): rewrites a managed copy at ext whose integration version is lower
// than this binary's. No copy, no change.
func UpdateDshExt(ext string) (bool, error) { return dshManaged.Update(ext) }

// RemoveDshExt deletes a managed copy at ext; it reports whether there was one.
func RemoveDshExt(ext string) (bool, error) { return dshManaged.Remove(ext) }

// DshWorkerPatchPath is the patch file (a dsh --patch overlay) the driver writes for its workers.
func DshWorkerPatchPath(dir string) string {
	return filepath.Join(PluginDir(dir, "dsh-worker"), "patch.yml")
}

// DshSessionsDir is where the plugin keeps the records of the web sessions it joins:
// <dir>/sessions/dsh/<session id>/records.jsonl (setup writes it into the plugin's row).
func DshSessionsDir(dir string) string { return SessionsRoot(dir, "dsh") }

var dshWorkerExt sync.Mutex

// EnsureDshWorker makes the plugin copy current and writes the workers' overlay, returning its
// path. Called before every spawn: the copy is written only when missing or not what this binary
// writes, never while current, so a worker that is starting is not raced by a rewrite.
func EnsureDshWorker(dir string, blacklist []string) (patch string, err error) {
	dshWorkerExt.Lock()
	defer dshWorkerExt.Unlock()
	if _, err := InstallDshExt(DshExtDir(dir)); err != nil {
		return "", err
	}
	patch = DshWorkerPatchPath(dir)
	if err := os.MkdirAll(filepath.Dir(patch), 0o700); err != nil {
		return "", err
	}
	tmp := patch + ".tmp"
	if err := os.WriteFile(tmp, []byte(dshWorkerPatch(DshEntry(dir), blacklist)), 0o600); err != nil {
		return "", err
	}
	return patch, os.Rename(tmp, patch)
}

// dshWorkerPatch is the overlay of a worker: the DeepSeek session-log upload off, the plugin
// under its own id (setup's row, when there is one, has another: the plugin loads once), and each
// blacklisted row of the Human's setup disabled.
func dshWorkerPatch(entry string, blacklist []string) string {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	var b strings.Builder
	b.WriteString("# written by piggery for its dsh workers; do not edit\n")
	b.WriteString("- id: session-log-deepseek\n  config:\n    enabled: false\n")
	for _, id := range blacklist {
		fmt.Fprintf(&b, "- id: %s\n  disabled: true\n", q(id))
	}
	fmt.Fprintf(&b, "- insert:\n    - id: piggery-worker\n      name: %s\n", q(entry))
	return b.String()
}
