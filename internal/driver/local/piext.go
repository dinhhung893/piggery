package local

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	piext "github.com/sting8k/piggery/extensions/pi"
)

// piggery's pi extension installed from the binary (managedext.go): the embedded files unpacked
// into <pi agent dir>/extensions/piggery, which pi loads by itself.

// piManaged is pi's extension as a managed copy: index.ts carries the marker.
var piManaged = managedExt{
	setup:      "pi",
	markerFile: "index.ts",
	hint:       "move it away, or use --ext",
	files: func() (map[string][]byte, error) {
		files := map[string][]byte{}
		err := fs.WalkDir(piext.Files, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := piext.Files.ReadFile(p)
			files[p] = b
			return err
		})
		return files, err
	},
}

// PiExtDir is where `piggery setup pi` installs the extension.
func PiExtDir(dir string) string {
	return filepath.Join(HumanAgentDir(AgentDirRoot(dir)), "extensions", "piggery")
}

// PiExtVersion is the version of the managed copy at ext; managed is false when ext holds none.
func PiExtVersion(ext string) (version string, managed bool) { return piManaged.Version(ext) }

// PiExtCurrent: the managed copy at ext holds exactly what version would write.
func PiExtCurrent(ext, version string) bool { return piManaged.Current(ext, version) }

// InstallPiExt writes the extension into ext, in place of a managed copy or of a symlink (an
// older way to add piggery). Anything else there is refused. It reports whether it wrote.
func InstallPiExt(ext, version string) (bool, error) { return piManaged.Install(ext, version) }

// UpdatePiExt (daemon start, like the templates): rewrites a managed copy at ext that is older
// than version, or, when either is a dev build, that differs from what version writes. No copy,
// no change.
func UpdatePiExt(ext, version string) (bool, error) { return piManaged.Update(ext, version) }

// RemovePiExt deletes a managed copy (or a symlink) at ext; it reports whether there was one.
func RemovePiExt(ext string) (bool, error) { return piManaged.Remove(ext) }

// parseSemver reads vX.Y.Z (a release); dev builds are not.
func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func semverLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
