package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// ps --json reads on from the cache: split anywhere (at each line, or inside one), a transcript
// read in two calls with the cache written and loaded in between gives the ctx and turns of one
// read from the start; a file that got shorter, or was rewritten, is read again from the start.
func TestLogCacheReadsOn(t *testing.T) {
	fixtures := map[string]string{
		"pi":     "../../testdata/fixtures/pi-0.87.1/session.jsonl",
		"claude": "../../testdata/fixtures/claude-2.1.283/transcript.jsonl",
		"codex":  "../../testdata/fixtures/codex-0.157.1/rollout.jsonl",
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "t.jsonl")
	// read is one ps --json call on path: from the cache, then saving it.
	read := func(format string) logState {
		t.Helper()
		st, ok := advanceLog(path, formats[format], loadLogCache(dir)[path])
		if !ok {
			t.Fatal("unreadable")
		}
		saveLogCache(dir, map[string]logState{path: st})
		return st
	}
	same := func(what string, got, want logState) {
		t.Helper()
		if got.turns != want.turns || got.tokens != want.tokens || got.seen != want.seen || got.off != want.off {
			t.Errorf("%s: turns %d ctx %d (%v) off %d, want %d %d (%v) %d", what, got.turns, got.tokens, got.seen, got.off,
				want.turns, want.tokens, want.seen, want.off)
		}
	}
	write := func(b []byte) {
		t.Helper()
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for format, fx := range fixtures {
		full, err := os.ReadFile(fx)
		if err != nil {
			t.Fatal(err)
		}
		os.RemoveAll(filepath.Join(dir, "cache"))
		write(full)
		want := read(format)

		var cuts []int
		for i, c := range full {
			if c == '\n' {
				cuts = append(cuts, i+1, i+1+(bytes.IndexByte(full[i+1:], '\n')+1)/2) // a line end, and inside the next
			}
		}
		for _, cut := range cuts {
			os.RemoveAll(filepath.Join(dir, "cache"))
			write(full[:cut])
			read(format)
			write(full)
			same(format+" read on from the cache", read(format), want)
		}

		// Shorter: the first half only, read again from the start as a new file would be.
		half := full[:bytes.LastIndexByte(full[:len(full)/2], '\n')+1]
		write(half)
		got := read(format)
		os.RemoveAll(filepath.Join(dir, "cache"))
		same(format+" after the file got shorter", got, read(format))
	}

	// Rewritten, not shorter (another file's bytes before the offset): read again from the start.
	pi, _ := os.ReadFile(fixtures["pi"])
	claude, _ := os.ReadFile(fixtures["claude"])
	os.RemoveAll(filepath.Join(dir, "cache"))
	write(pi)
	read("pi")
	write(append(append([]byte{}, claude...), pi...)) // longer, and different before the old offset
	got := read("pi")
	os.RemoveAll(filepath.Join(dir, "cache"))
	same("after a rewrite", got, read("pi"))
}
