package local

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	ompext "github.com/sting8k/piggery/extensions/omp"
)

// piggery's omp extension installed from the binary (managedext.go), in two places:
//
//   - the Human's: <omp agent dir>/extensions/piggery, which omp loads by itself (`piggery setup omp`);
//   - a worker's: a copy piggery owns, <dir>/plugins/omp, loaded with `-e <entry>`
//     (EnsureOmpWorkerExt), so a worker needs no `setup omp` and never depends on the Human's copy.
//
// Both hold the tree of ompext.Tree: omp's manifest, omp/index.ts and pi/<shared files>.

// ompManaged is omp's extension as a managed copy: omp/index.ts carries the marker.
var ompManaged = managedExt{
	setup:      "omp",
	markerFile: ompext.Entry,
	hint:       "move it away",
	files:      ompext.Tree,
}

// OmpAgentDirRoot is where per-run omp agent dirs live: <dir>/run/omp/<participant>/<run>
// (the driver builds them, like pi's AgentDirRoot).
func OmpAgentDirRoot(dir string) string { return RunRoot(dir, "omp") }

// ompProfileName is omp's profile name rule (packages/utils/src/dirs.ts).
var ompProfileName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// ompProfile is the profile omp runs under: OMP_PROFILE, else PI_PROFILE (an OMP_PROFILE set,
// even empty, wins); "" is the default profile. An invalid name is the default one, as omp's own
// module-load resolution does.
func ompProfile() string {
	v, ok := os.LookupEnv("OMP_PROFILE")
	if !ok {
		v = os.Getenv("PI_PROFILE")
	}
	v = strings.TrimSpace(v)
	if v == "" || v == "default" || v == "." || v == ".." || strings.HasSuffix(v, ".") || !ompProfileName.MatchString(v) {
		return ""
	}
	return v
}

// OmpHumanAgentDir is the agent dir the Human's omp uses: the profile's, else PI_CODING_AGENT_DIR,
// else ~/.omp/agent (PI_CONFIG_DIR renames ~/.omp). A named profile ignores PI_CODING_AGENT_DIR,
// as omp does. A PI_CODING_AGENT_DIR under root (a daemon started from inside a worker inherits
// the worker's generated dir) is ignored: a worker dir is never built from another.
func OmpHumanAgentDir(root string) string {
	home, _ := os.UserHomeDir()
	base := os.Getenv("PI_CONFIG_DIR")
	if base == "" {
		base = ".omp"
	}
	if !filepath.IsAbs(base) {
		base = filepath.Join(home, base)
	}
	if p := ompProfile(); p != "" {
		return filepath.Join(base, "profiles", p, "agent")
	}
	if d := os.Getenv("PI_CODING_AGENT_DIR"); d != "" {
		d = normalizePath(d, home, home)
		if abs, err := filepath.Abs(d); err == nil && !within(abs, root) {
			return abs
		}
	}
	return filepath.Join(base, "agent")
}

// OmpExtDir is where `piggery setup omp` installs the extension.
func OmpExtDir(dir string) string {
	return filepath.Join(OmpHumanAgentDir(OmpAgentDirRoot(dir)), "extensions", "piggery")
}

// OmpExtVersion is the integration version of the managed copy at ext; managed is false when ext
// holds none.
func OmpExtVersion(ext string) (version int, managed bool) { return ompManaged.Version(ext) }

// OmpExtCurrent: the managed copy at ext holds exactly what this binary would write.
func OmpExtCurrent(ext string) bool { return ompManaged.Current(ext) }

// InstallOmpExt writes the extension into ext, in place of a managed copy. Anything else there is
// refused. It reports whether it wrote.
func InstallOmpExt(ext string) (bool, error) { return ompManaged.Install(ext) }

// UpdateOmpExt (daemon start): rewrites a managed copy at ext whose integration version is lower
// than this binary's. No copy, no change.
func UpdateOmpExt(ext string) (bool, error) { return ompManaged.Update(ext) }

// RemoveOmpExt deletes a managed copy at ext; it reports whether there was one.
func RemoveOmpExt(ext string) (bool, error) { return ompManaged.Remove(ext) }

// OmpWorkerExtDir is the copy piggery owns for its omp workers.
func OmpWorkerExtDir(dir string) string { return PluginDir(dir, "omp") }

var ompWorkerExt sync.Mutex

// EnsureOmpWorkerExt makes the workers' copy current and returns the file to load
// with `omp -e <entry>` (an explicit -e loads even with --no-extensions, and the per-run agent dir's
// empty extensions/ leaves nothing else to load twice). Called before every spawn: it writes only
// when the copy is missing or not what this binary writes, never while it is current, so a
// worker that is starting is not raced by a rewrite; a running worker has its files loaded already.
func EnsureOmpWorkerExt(dir string) (entry string, err error) {
	ompWorkerExt.Lock()
	defer ompWorkerExt.Unlock()
	ext := OmpWorkerExtDir(dir)
	if _, err := InstallOmpExt(ext); err != nil {
		return "", err
	}
	return filepath.Join(ext, filepath.FromSlash(ompext.Entry)), nil
}
