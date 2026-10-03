package cli

import (
	"bytes"
	"os"
	"slices"
	"testing"
)

// replay reads a captured transcript through its format's reader, as top and tail do, plus a
// line no reader understands: the context now, the turns and the tail lines.
func replay(t *testing.T, format, path string) (ctx, turns int, lines []string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := formats[format]()
	for _, rec := range append(bytes.Split(bytes.TrimSpace(b), []byte("\n")), []byte(`not json`), []byte(`{"type":"new-kind"}`)) {
		x := r.read(rec)
		if x.hasCtx {
			ctx = x.ctx
		}
		if x.turnEnd {
			turns++
		}
		if x.line != "" {
			lines = append(lines, x.line)
		}
	}
	return ctx, turns, lines
}

// A real Claude Code transcript (testdata/fixtures/claude-2.1.283/transcript.jsonl): the
// latest reply's context (input, cache and output), one turn per reply though a reply spans
// several lines (thinking, text), and a tail of the prompts, replies and the tool call with its
// named result; the text Claude adds itself (isMeta: hook feedback, mail) is not in it.
func TestClaudeTranscriptReplay(t *testing.T) {
	ctx, turns, lines := replay(t, "claude", "../../testdata/fixtures/claude-2.1.283/transcript.jsonl")
	if ctx != 18062 || turns != 5 {
		t.Errorf("ctx %d turns %d, want 18062 and 5", ctx, turns)
	}
	for _, want := range []string{"user: Reply with exactly: one", "assistant: one", "> Bash sleep 3",
		"< Bash ok: (Bash completed with no output)"} {
		if !slices.Contains(lines, want) {
			t.Errorf("tail has no %q: %q", want, lines)
		}
	}
	for _, l := range lines {
		if bytes.Contains([]byte(l), []byte("GIRAFFE before")) || bytes.Contains([]byte(l), []byte("nudge #1")) {
			t.Errorf("text Claude added is in the tail: %q", l)
		}
	}
}

// A real Codex rollout (testdata/fixtures/codex-0.157.1/rollout.jsonl): context and turns from
// the token_count events, and a tail of the prompt, the tool call with its output and the reply;
// the context Codex adds (developer messages, <environment_context>) is not in it.
func TestCodexRolloutReplay(t *testing.T) {
	ctx, turns, lines := replay(t, "codex", "../../testdata/fixtures/codex-0.157.1/rollout.jsonl")
	if ctx != 13375 || turns != 2 {
		t.Errorf("ctx %d turns %d, want 13375 and 2", ctx, turns)
	}
	if len(lines) != 4 || !bytes.HasPrefix([]byte(lines[0]), []byte("user: Run the shell command")) ||
		!bytes.HasPrefix([]byte(lines[1]), []byte("> exec const r = await tools.exec_command")) ||
		!bytes.HasPrefix([]byte(lines[2]), []byte("< exec ok: Script completed")) || lines[3] != "assistant: NESTED-OK" {
		t.Errorf("tail %q", lines)
	}
}

// A real pi session file (testdata/fixtures/pi-0.87.1/session.jsonl, system prompt removed): the
// latest assistant message's context, one turn per assistant message, and a tail of the mail
// prompt, each tool call with its named result and the reply; headers, the extension's custom
// message and the system prompt show nothing.
func TestPiSessionReplay(t *testing.T) {
	ctx, turns, lines := replay(t, "pi", "../../testdata/fixtures/pi-0.87.1/session.jsonl")
	if ctx != 8406 || turns != 3 {
		t.Errorf("ctx %d turns %d, want 8406 and 3", ctx, turns)
	}
	want := []string{"> bash sleep 20", "< bash ok: exit 0 bg-1 · sleep 20 · done (no output)", "> piggery_send",
		"< piggery_send ok: sent #2 (thread #1)", `assistant: Done — ran ` + "`sleep 20`" + ` and replied "ok" to lead.`}
	if len(lines) != 6 || !bytes.HasPrefix([]byte(lines[0]), []byte("user: [piggery] 1 new message:")) {
		t.Fatalf("tail %q", lines)
	}
	for i, w := range want {
		if !bytes.HasPrefix([]byte(lines[i+1]), []byte(w)) {
			t.Errorf("tail[%d] = %q, want %q...", i+1, lines[i+1], w)
		}
	}
}

// omp's session file has pi's format (message records with usage, plus title and custom records), so
// the pi reader reads it as is, which is why the omp extension reports format "pi"
// (testdata/fixtures/omp/session-file-tool-call.jsonl, a real omp 18.4.2 session): the context of the
// last assistant message, one turn per assistant message, and the tail with the named tool result.
func TestOmpSessionFileReplay(t *testing.T) {
	ctx, turns, lines := replay(t, "pi", "../../testdata/fixtures/omp/session-file-tool-call.jsonl")
	if ctx != 7801 || turns != 2 || len(lines) != 4 || !bytes.HasPrefix([]byte(lines[0]), []byte("user: Call the piggery_who tool")) ||
		!bytes.HasPrefix([]byte(lines[2]), []byte("< piggery_who ok: ")) || !bytes.HasPrefix([]byte(lines[3]), []byte("assistant: Only one solo session")) {
		t.Fatalf("ctx %d turns %d tail %q", ctx, turns, lines)
	}
}
