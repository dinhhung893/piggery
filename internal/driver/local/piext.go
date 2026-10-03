package local

import (
	"io/fs"
	"path/filepath"

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

// PiExtVersion is the integration version of the managed copy at ext; managed is false when ext
// holds none.
func PiExtVersion(ext string) (version int, managed bool) { return piManaged.Version(ext) }

// PiExtCurrent: the managed copy at ext holds exactly what this binary would write.
func PiExtCurrent(ext string) bool { return piManaged.Current(ext) }

// InstallPiExt writes the extension into ext, in place of a managed copy or of a symlink (an
// older way to add piggery). Anything else there is refused. It reports whether it wrote.
func InstallPiExt(ext string) (bool, error) { return piManaged.Install(ext) }

// UpdatePiExt (daemon start, like the templates): rewrites a managed copy at ext whose
// integration version is lower than this binary's. No copy, no change.
func UpdatePiExt(ext string) (bool, error) { return piManaged.Update(ext) }

// RemovePiExt deletes a managed copy (or a symlink) at ext; it reports whether there was one.
func RemovePiExt(ext string) (bool, error) { return piManaged.Remove(ext) }
