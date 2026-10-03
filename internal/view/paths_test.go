package view

import (
	"slices"
	"testing"
)

// A cwd column shows a path's last two folders; two paths whose tails read the same get more
// folders until they differ; a short path stays whole.
func TestShortPathsTellApart(t *testing.T) {
	got := ShortPaths([]string{"/srv/a/x/proj/api", "/srv/b/x/proj/api", "/srv/c/other", "/tmp/x"})
	want := []string{"…/a/x/proj/api", "…/b/x/proj/api", "…/c/other", "/tmp/x"}
	if !slices.Equal(got, want) {
		t.Fatalf("ShortPaths = %q; want %q", got, want)
	}
}
