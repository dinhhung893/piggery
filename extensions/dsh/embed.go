// Package dshext is piggery's dsh (DeepSeek Harness) plugin as shipped in the binary: `piggery setup
// dsh` unpacks it under ~/.piggery/dsh, and dsh loads its entry as a plugin, for the sessions the
// Human opens (`dsh web`) and for the workers piggery spawns (`dsh --profile sdk`) alike.
//
// The entry (dsh/index.mjs) is dsh's own; it reuses pi's adapter, client, render and tools.json by
// import (../pi/<file>), so there is one copy of each in the repository. An unpacked copy has the
// same shape as the repository, so the imports resolve in both:
//
//	dsh/<module>      this directory's modules
//	pi/<shared file>  the files of extensions/pi that dsh/index.mjs imports
package dshext

import (
	"embed"
	"fmt"

	piext "github.com/sting8k/piggery/extensions/pi"
)

//go:embed index.mjs bridge.mjs records.mjs
var files embed.FS

// Entry is the path, from the root of an unpacked copy, of the file dsh loads.
const Entry = "dsh/index.mjs"

// Own are this directory's modules, at dsh/<name> in a copy.
var Own = []string{"index.mjs", "bridge.mjs", "records.mjs"}

// Shared are the files of extensions/pi that dsh/index.mjs imports, at pi/<name> in a copy. A test
// keeps this list equal to the imports.
var Shared = []string{"adapter.mjs", "client.mjs", "render.mjs", "tools.json"}

// Tree is the files of a copy by slash path: the plugin's modules and the shared files.
func Tree() (map[string][]byte, error) {
	tree := map[string][]byte{}
	for _, name := range Own {
		b, err := files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("dshext: %s: %w", name, err)
		}
		tree["dsh/"+name] = b
	}
	for _, name := range Shared {
		b, err := piext.Files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("dshext: shared file %s: %w", name, err)
		}
		tree["pi/"+name] = b
	}
	return tree, nil
}
