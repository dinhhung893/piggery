// Package ompext is piggery's omp (oh-my-pi) extension as shipped in the binary: `piggery setup omp`
// unpacks it into omp's extensions directory, and an omp worker loads the same files with -e.
//
// The entry (index.ts) is omp's own; it reuses pi's adapter, client, render and tools.json by
// import (../pi/<file>), so there is one copy of each in the repository. An unpacked copy has the
// same shape as the repository, so the imports resolve in both:
//
//	package.json      omp's manifest, naming ./omp/index.ts as the only entry
//	omp/index.ts      this directory's index.ts
//	pi/<shared file>  the files of extensions/pi that omp/index.ts imports
package ompext

import (
	_ "embed"
	"fmt"

	piext "github.com/sting8k/piggery/extensions/pi"
)

//go:embed index.ts
var entry []byte

// Entry is the path, from the root of an unpacked copy, of the file omp loads.
const Entry = "omp/index.ts"

// PackageName is the name in the copy's package.json: what tells piggery's copy from another
// extension in the same directory.
const PackageName = "piggery-omp"

// manifest is the copy's package.json. omp reads `omp` before `pi`, and a manifest is
// authoritative: only the listed entry loads, never the shared files beside it.
const manifest = `{
  "name": "` + PackageName + `",
  "private": true,
  "type": "module",
  "description": "piggery adapter for omp",
  "omp": {
    "extensions": ["./` + Entry + `"]
  }
}
`

// Shared are the files of extensions/pi that omp/index.ts imports, at pi/<name> in a copy. A test
// keeps this list equal to the imports.
var Shared = []string{"adapter.mjs", "client.mjs", "render.mjs", "tools.json"}

// Tree is the files of a copy by slash path: the manifest, the entry, and the shared files.
func Tree() (map[string][]byte, error) {
	files := map[string][]byte{
		"package.json": []byte(manifest),
		Entry:          append([]byte(nil), entry...),
	}
	for _, name := range Shared {
		b, err := piext.Files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("ompext: shared file %s: %w", name, err)
		}
		files["pi/"+name] = b
	}
	return files, nil
}
