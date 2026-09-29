package local

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A managed extension is one of piggery's own extensions written from the binary into a
// directory a harness loads by itself (pi: <agent dir>/extensions/piggery). The first line of
// its marker file says it is piggery's and names the version that wrote it; a directory there
// without that line is not piggery's to replace or remove. pi and omp differ only in what the
// files are, which file carries the marker, and the words of the messages.
type managedExt struct {
	setup      string                            // `piggery setup <setup>` writes it
	markerFile string                            // slash path (from the copy's root) of the file whose first line is the marker
	hint       string                            // what to do with a directory that is not piggery's
	files      func() (map[string][]byte, error) // the files as the binary carries them, by slash path
}

const managedMarker = "// managed by piggery "

// written are the files of a copy written by version: the marker line goes in front of the marker file.
func (m managedExt) written(version string) (map[string][]byte, error) {
	files, err := m.files()
	if err != nil {
		return nil, err
	}
	line := managedMarker + version + ": written by `piggery setup " + m.setup + "`; run it again or `piggery setup remove " + m.setup + "` instead of editing\n"
	files[m.markerFile] = append([]byte(line), files[m.markerFile]...)
	return files, nil
}

// Version is the version of the managed copy at ext; managed is false when ext holds none.
func (m managedExt) Version(ext string) (version string, managed bool) {
	b, err := os.ReadFile(filepath.Join(ext, filepath.FromSlash(m.markerFile)))
	if err != nil || !bytes.HasPrefix(b, []byte(managedMarker)) {
		return "", false
	}
	line, _, _ := strings.Cut(string(b[len(managedMarker):]), "\n")
	v, _, _ := strings.Cut(line, ":")
	return v, true
}

// Current: the copy at ext holds exactly what version would write (no file more, none less).
func (m managedExt) Current(ext, version string) bool {
	files, err := m.written(version)
	if err != nil {
		return false
	}
	n := 0
	err = filepath.WalkDir(ext, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(ext, p)
		if err != nil {
			return err
		}
		want, ok := files[filepath.ToSlash(rel)]
		got, rerr := os.ReadFile(p)
		if !ok || rerr != nil || !bytes.Equal(got, want) {
			return errors.New("differs")
		}
		n++
		return nil
	})
	return err == nil && n == len(files)
}

// Install writes the extension into ext, in place of a managed copy or of a symlink (an older way
// to add piggery). Anything else there is refused. It reports whether it wrote.
func (m managedExt) Install(ext, version string) (bool, error) {
	if fi, err := os.Lstat(ext); err == nil {
		if _, managed := m.Version(ext); fi.Mode()&os.ModeSymlink == 0 && !managed {
			return false, fmt.Errorf("%s is not piggery's (no %q line): %s", ext, strings.TrimSpace(managedMarker), m.hint)
		}
		if m.Current(ext, version) {
			return false, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	files, err := m.written(version)
	if err != nil {
		return false, err
	}
	tmp := ext + ".piggery-tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return false, err
	}
	for name, b := range files {
		p := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return false, err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return false, err
		}
	}
	if err := os.RemoveAll(ext); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, ext)
}

// Update (daemon start, like the templates): rewrites a managed copy at ext that is older than
// version, or, when either is a dev build, that differs from what version writes. No copy, no
// change.
func (m managedExt) Update(ext, version string) (bool, error) {
	have, managed := m.Version(ext)
	if !managed || m.Current(ext, version) {
		return false, nil
	}
	if a, okA := parseSemver(have); okA {
		if b, okB := parseSemver(version); okB && !semverLess(a, b) {
			return false, nil // the copy is as new or newer: another binary wrote it
		}
	}
	return m.Install(ext, version)
}

// Remove deletes a managed copy (or a symlink) at ext; it reports whether there was one.
func (m managedExt) Remove(ext string) (bool, error) {
	fi, err := os.Lstat(ext)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if _, managed := m.Version(ext); fi.Mode()&os.ModeSymlink == 0 && !managed {
		return false, fmt.Errorf("%s is not piggery's: left as it is", ext)
	}
	return true, os.RemoveAll(ext)
}
