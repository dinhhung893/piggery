package cli

import (
	"fmt"
	"os"

	"github.com/sting8k/piggery/internal/driver/local"
)

// omp: piggery's extension is the copy the binary carries, unpacked into omp's
// <agent dir>/extensions/piggery (omp loads it by itself; its manifest names the one entry). There
// is no checkout way (pi's --ext): a checkout is `omp -e extensions/omp/index.ts`. The omp worker
// profile (~/.piggery/harness/omp.json) is written by `piggery setup` and the daemon; workers load
// their own copy (local.EnsureOmpWorkerExt), never this one.

// ompHarness: omp's extension talks to the daemon itself (no hooks, no piggery mcp).
var ompHarness = harnessProfile{
	setupTarget: setupTarget{name: "omp", cmd: "omp",
		install: func(o setupOpts) (string, error) { return installOmp(o.dir, Version) },
		remove:  func(o setupOpts) (string, error) { return removeOmp(o.dir) },
		status:  func(o setupOpts) harnessState { return ompStatus(o.dir, o.self, Version) },
	},
	profilePath: local.OmpProfilePath,
}

func installOmp(dir, version string) (string, error) {
	dst := local.OmpExtDir(dir)
	wrote, err := local.InstallOmpExt(dst, version)
	if err != nil {
		return "", err
	}
	if !wrote {
		return "omp: piggery's extension is already installed", nil
	}
	return fmt.Sprintf("omp: installed piggery's extension (%s) in %s\nomp: sessions started from now on join piggery; restart any that are open.", version, dst), nil
}

func removeOmp(dir string) (string, error) {
	ext := local.OmpExtDir(dir)
	if removed, err := local.RemoveOmpExt(ext); err != nil {
		return "", err
	} else if !removed {
		return "omp: piggery is not installed", nil
	}
	return "omp: removed " + ext, nil
}

// ompStatus: the installed copy (current for this binary), and `piggery` on PATH being this binary
// (the extension starts the daemon with it). A directory there that is not piggery's is reported.
func ompStatus(dir, self, version string) harnessState {
	st := harnessState{Name: "omp"}
	ext := local.OmpExtDir(dir)
	have, managed := local.OmpExtVersion(ext)
	if !managed {
		if _, err := os.Lstat(ext); err == nil {
			st.Problems = append(st.Problems, problem{ext + " is not piggery's (no \"managed by piggery\" line): setup omp will not replace it", "move it away, then `piggery setup omp`"})
		}
		return st
	}
	st.Installed, st.Detail = true, ext+" ("+have+")"
	if !local.OmpExtCurrent(ext, version) {
		st.Problems = append(st.Problems, problem{fmt.Sprintf("the installed copy (%s) is not this piggery's (%s)", have, version), "piggery setup omp"})
	}
	st.Problems = append(st.Problems, piggeryOnPath(self)...)
	return st
}
