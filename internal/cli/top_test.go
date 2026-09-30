package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sting8k/piggery/internal/core"
	"github.com/sting8k/piggery/internal/driver/local"
	"github.com/sting8k/piggery/internal/proto"
)

// A worker's context in top is the usage of its latest assistant message_end in its current
// run, not a sum; its turns count turn_end across all its runs; refreshes read only what the
// logs gained.
func TestTopWorkerStats(t *testing.T) {
	dir := t.TempDir()
	ps := proto.PsResult{State: core.State{Teams: []core.TeamState{{Members: []core.MemberState{
		{ID: "w1", RunID: "r1", Headless: true, Capabilities: []string{core.CapUsage}}}}}}}
	path := local.LogPath(dir, "w1", "r1")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		f.WriteString(s)
	}
	write(`{"type":"message_end","message":{"role":"assistant","usage":{"input":857,"output":142,"cacheRead":14464,"cacheWrite":0,"totalTokens":15463}}}
{"type":"turn_end"}
{"type":"message_end","message":{"role":"user"}}
{"type":"message_end","message":{"role":"assistant","usage":{"input":900,"output":100,"cacheRead":15000,"cacheWrite":0,"totalTokens":16000}}}
`)
	old := local.LogPath(dir, "w1", "r0") // an earlier run: its turns count, its usage does not
	if err := os.WriteFile(old, []byte(`{"type":"turn_end"}
{"type":"message_end","message":{"role":"assistant","usage":{"totalTokens":99999}}}
{"type":"turn_end"}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &topModel{dir: dir, ps: ps}
	m.logs = readLogs(dir, ps, nil)
	if got := m.stats()["w1"]; got.ctx != 16000 || got.turns != 3 {
		t.Fatalf("stats = %+v; want ctx 16000 (latest of the current run, not a sum), turns 3 (both runs)", got)
	}
	write(`{"type":"message_end","message":{"role":"assistant","usage":{"input":10,"output":5,"cacheRead":2000,"cacheWrite":0}}}
{"type":"turn_end"}
`)
	m.logs = readLogs(dir, ps, m.logs)
	if got := m.stats()["w1"]; got.ctx != 2015 || got.turns != 4 {
		t.Fatalf("stats after append = %+v; want ctx 2015 (no totalTokens: the sum of its parts), turns 4", got)
	}
}

// Switching the tail from w1 to w2 never shows w1's lines under w2, also while the fetch in
// flight still carries w1's tail.
func TestTopTailFollowsTheSelection(t *testing.T) {
	ps := proto.PsResult{State: core.State{Teams: []core.TeamState{{Name: "t", Members: []core.MemberState{
		{ID: "w1", Name: "w1", Headless: true, Capabilities: []string{core.CapUsage}}, {ID: "w2", Name: "w2", Headless: true, Capabilities: []string{core.CapUsage}}}}}}}
	m := newTopModel(nil, "")
	m.Update(fetched{ps: ps})
	m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	m.Update(fetched{ps: ps, tail: tailState{worker: "w1", lines: []string{"from w1"}}})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.sel != "w2" || !m.side || m.sideTab != sideTail {
		t.Fatalf("sel %q side %v tab %d; want w2's tail", m.sel, m.side, m.sideTab)
	}
	m.Update(fetched{ps: ps, tail: tailState{worker: "w1", lines: []string{"from w1", "more from w1"}}})
	if out := m.render(); strings.Contains(out, "from w1") {
		t.Fatalf("w2's tail shows w1's lines:\n%s", out)
	}
}

// Tabs are All, each team (Closed last). Switching tabs moves the selection into the new tab; a
// refresh keeps it while the row is there, else takes the tab's first row; a tab whose team
// closed falls back to All.
func TestTopTabsKeepASelection(t *testing.T) {
	ps := proto.PsResult{State: core.State{
		Teams: []core.TeamState{
			{ID: "ta", Name: "a", Members: []core.MemberState{{ID: "a1", Name: "a1"}, {ID: "a2", Name: "a2"}}},
			{ID: "tb", Name: "b", Members: []core.MemberState{{ID: "b1", Name: "b1"}}}},
		Solos: []core.SoloState{{ID: "s1", Name: "s1"}}}}
	m := newTopModel(nil, "")
	m.Update(fetched{ps: ps})
	right := tea.KeyPressMsg{Code: tea.KeyRight}
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.tab != "" || m.sel != "a2" {
		t.Fatalf("start: tab %q sel %q; want All, a2", m.tab, m.sel)
	}
	m.Update(fetched{ps: ps})
	if m.sel != "a2" {
		t.Fatalf("refresh moved the selection to %q", m.sel)
	}
	var got []string
	for range 3 {
		m.Update(right)
		got = append(got, m.tab+":"+m.sel)
	}
	if want := "ta:a2 tb:b1 :b1"; strings.Join(got, " ") != want {
		t.Fatalf("tabs: got %q want %q", strings.Join(got, " "), want)
	}
	m.Update(right) // team a
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	gone := ps
	gone.Teams = []core.TeamState{{ID: "ta", Name: "a", Members: []core.MemberState{{ID: "a1", Name: "a1"}}}, ps.Teams[1]}
	m.Update(fetched{ps: gone})
	if m.tab != "ta" || m.sel != "a1" {
		t.Fatalf("a2 left: tab %q sel %q; want ta, a1", m.tab, m.sel)
	}
	gone.Teams = gone.Teams[1:]
	m.Update(fetched{ps: gone})
	if m.tab != "" || m.sel != "b1" {
		t.Fatalf("team a closed: tab %q sel %q; want All, b1", m.tab, m.sel)
	}
}

// A click lands on what the frame drew there: on a member's row it selects that member, on a
// tab it switches to that tab.
func TestTopClickSelectsWhatWasDrawn(t *testing.T) {
	ps := proto.PsResult{State: core.State{
		Teams: []core.TeamState{
			{ID: "ta", Name: "a", Members: []core.MemberState{{ID: "a1", Name: "a1"}, {ID: "a2", Name: "a2"}}},
			{ID: "tb", Name: "b", Members: []core.MemberState{{ID: "b1", Name: "b1"}}}}}}
	m := newTopModel(nil, "")
	m.w, m.h = 120, 30
	m.Update(fetched{ps: ps})
	find := func(text string) (int, int) {
		sgr := regexp.MustCompile("\x1b\\[[0-9;]*m")
		for y, l := range strings.Split(sgr.ReplaceAllString(m.render(), ""), "\n") {
			if i := strings.Index(l, text); i >= 0 {
				return lipgloss.Width(l[:i]), y
			}
		}
		t.Fatalf("%q not drawn", text)
		return 0, 0
	}
	x, y := find("a2 ")
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if m.sel != "a2" {
		t.Fatalf("click on a2's row selected %q", m.sel)
	}
	x, y = find("b 1")
	m.Update(tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	if m.tab != "tb" || m.sel != "b1" {
		t.Fatalf("click on tab b: tab %q sel %q", m.tab, m.sel)
	}
}

// A closed team is listed as one line: in All while it closed within the hour, in Closed until gc
// removes it; enter on its line expands it into its members, which can then be selected.
func TestTopClosedTeamsExpand(t *testing.T) {
	now := time.Now()
	ps := proto.PsResult{State: core.State{
		Teams: []core.TeamState{{ID: "ta", Name: "a", Members: []core.MemberState{{ID: "a1", Name: "a1"}}}},
		Closed: []core.ClosedTeam{
			{TeamState: core.TeamState{ID: "tr", Name: "recent", Members: []core.MemberState{{ID: "r1", Name: "r1", State: "gone"}}},
				ClosedAt: now.Add(-10 * time.Minute).UnixMilli()},
			{TeamState: core.TeamState{ID: "to", Name: "old", Members: []core.MemberState{{ID: "o1", Name: "o1", State: "gone"}}},
				ClosedAt: now.Add(-2 * time.Hour).UnixMilli()}}}}
	m := newTopModel(nil, "")
	m.Update(fetched{ps: ps})
	if got, want := strings.Join(m.items(), " "), closedRow+"ta a1 "+closedRow+"tr"; got != want {
		t.Fatalf("All lists %q; want the open team's line and member, and the recent closed team, collapsed", got)
	}
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got, want := strings.Join(m.items(), " "), closedRow+"ta a1 "+closedRow+"tr r1"; got != want {
		t.Fatalf("after enter on the closed team All lists %q; want its member r1 too", got)
	}
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.sel != "r1" {
		t.Fatalf("selected %q; want r1, a member of the expanded closed team", m.sel)
	}
	for m.tab != tabClosed {
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if got, want := strings.Join(m.items(), " "), closedRow+"tr r1 "+closedRow+"to"; got != want {
		t.Fatalf("Closed lists %q; want every closed team, newest first", got)
	}
}

// An open team whose members are all gone is one line in All, collapsed until enter; its own
// tab lists its members; when a member is back it is a normal team, no key needed.
func TestTopDeadOpenTeamIsOneLine(t *testing.T) {
	ps := func(d2 string) proto.PsResult {
		return proto.PsResult{State: core.State{Teams: []core.TeamState{
			{ID: "ta", Name: "a", Members: []core.MemberState{{ID: "a1", Name: "a1", State: "idle"}}},
			{ID: "td", Name: "dead", Members: []core.MemberState{{ID: "d1", Name: "d1", State: "gone"}, {ID: "d2", Name: "d2", State: d2}}}}}}
	}
	m := newTopModel(nil, "")
	m.Update(fetched{ps: ps("gone")})
	if got, want := strings.Join(m.items(), " "), closedRow+"ta a1 "+closedRow+"td"; got != want {
		t.Fatalf("All lists %q; want the live member and the dead team as one line", got)
	}
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got, want := strings.Join(m.items(), " "), closedRow+"ta a1 "+closedRow+"td d1 d2"; got != want {
		t.Fatalf("after enter All lists %q; want the dead team's members too", got)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for m.tab != "td" {
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if got, want := strings.Join(m.items(), " "), "d1 d2"; got != want {
		t.Fatalf("its own tab lists %q; want its members, no collapsed line", got)
	}
	m.tab = ""
	m.Update(fetched{ps: ps("idle")})
	if got, want := strings.Join(m.items(), " "), closedRow+"ta a1 "+closedRow+"td d2 "+goneRow+"td"; got != want {
		t.Fatalf("with d2 back All lists %q; want a normal team, d1 folded as gone", got)
	}
}

// On a narrow terminal the details start hidden; enter shows them instead of the list and esc
// goes back to the list with the same member selected.
func TestTopNarrowDetailsFullScreen(t *testing.T) {
	ps := proto.PsResult{State: core.State{Teams: []core.TeamState{{ID: "ta", Name: "a", Members: []core.MemberState{
		{ID: "a1", Name: "a1"}, {ID: "a2", Name: "a2"}}}}}}
	m := newTopModel(nil, "")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(fetched{ps: ps})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.sideShown() || m.sel != "a2" {
		t.Fatalf("narrow start: details shown %v, sel %q; want hidden, a2", m.sideShown(), m.sel)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.full || !m.sideShown() {
		t.Fatal("enter did not show the details full screen")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.sideShown() || m.sel != "a2" {
		t.Fatalf("after esc: details shown %v, sel %q; want the list, a2 still selected", m.sideShown(), m.sel)
	}
}

// Tool output in worker logs carries terminal escapes; a tail line never passes them on.
func TestTailLineDropsControlSequences(t *testing.T) {
	rec := `{"type":"tool_execution_end","toolName":"bash","result":{"content":[{"type":"text","text":"HTTP/1.1 200 OK\r\n\u001b[31m\u001b[1mWARNING: dev server\u001b[0m\u001b]0;title\u0007 done\u0008"}]}}`
	l, ok := tailLine([]byte(rec))
	if !ok || l != "< bash ok: HTTP/1.1 200 OK WARNING: dev server done" {
		t.Fatalf("tail line = %q", l)
	}
}

// A tool line shows what the call does: bash its command, edit its path, another tool its
// arguments as key=value.
func TestTailLineShortensToolArgs(t *testing.T) {
	for rec, want := range map[string]string{
		`{"type":"tool_execution_start","toolName":"bash","args":{"command":"go test ./...","timeout":60}}`:        "> bash go test ./...",
		`{"type":"tool_execution_start","toolName":"edit","args":{"path":"internal/core/x.go","edits":[{"a":1}]}}`: "> edit internal/core/x.go",
		`{"type":"tool_execution_start","toolName":"grep","args":{"pattern":"TODO","path":"internal","limit":5}}`:  "> grep limit=5 path=internal pattern=TODO",
	} {
		if l, ok := tailLine([]byte(rec)); !ok || l != want {
			t.Fatalf("tail line = %q, want %q", l, want)
		}
	}
}

// Members show as the tree of reports_to: every parent before its children, each child under
// its own parent in team order; a member whose parent left the team is a root.
func TestMemberTree(t *testing.T) {
	ms := []core.MemberState{
		{ID: "s", Name: "s"},
		{ID: "lead-a", Name: "lead-a", ReportsTo: "s"},
		{ID: "lead-b", Name: "lead-b", ReportsTo: "s", State: "gone"},
		{ID: "peer-a1", Name: "peer-a1", ReportsTo: "lead-a"},
		{ID: "peer-b1", Name: "peer-b1", ReportsTo: "lead-b"},
		{ID: "peer-a2", Name: "peer-a2", ReportsTo: "lead-a"},
		{ID: "orphan", Name: "orphan", ReportsTo: "left-the-team"},
	}
	var got []string
	for _, r := range memberTree(ms) {
		got = append(got, fmt.Sprintf("%d %s<-%s", r.depth, r.m.Name, r.m.ReportsTo))
	}
	want := []string{"0 s<-", "1 lead-a<-s", "2 peer-a1<-lead-a", "2 peer-a2<-lead-a", "1 lead-b<-s",
		"2 peer-b1<-lead-b", "0 orphan<-left-the-team"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tree order:\n got %v\nwant %v", got, want)
	}
}

// A member's task sits right under the header, before team/kind/model: its title on at most two
// lines, who gave it, how its reply chain stands, and a newer unmarked mail as a "mail" row.
func TestTopOverviewShowsTheAssignment(t *testing.T) {
	now := time.UnixMilli(10_000_000)
	ago := func(min int) int64 { return now.Add(-time.Duration(min) * time.Minute).UnixMilli() }
	long := "omp phase 2, your part: the interactive side of the extension with a very long title that goes past two lines of the pane"
	a := &core.Assignment{Seq: 175, Title: long, From: "summer-hamster", At: ago(22),
		Latest: &core.ChainMail{Seq: 183, At: ago(3), ByMember: true}, Newer: &core.NewerMail{Seq: 190, Title: "next: the dsh part", At: ago(1)}}
	m := newTopModel(nil, "")
	m.Update(fetched{ps: proto.PsResult{State: core.State{Teams: []core.TeamState{{ID: "ta", Name: "a", Members: []core.MemberState{
		{ID: "a1", Name: "a1", Role: "executor", State: "idle", Assignment: a}, {ID: "a2", Name: "a2", Role: "executor", State: "idle"}}}}}}})
	strip := regexp.MustCompile("\x1b\\[[0-9;]*m")
	side := func(sel string) string {
		m.sel = sel
		return strip.ReplaceAllString(strings.Join(m.sidebar(50, 30, now), "\n"), "")
	}
	got := strings.Split(side("a1"), "\n")
	if len(got) < 8 || !strings.HasPrefix(got[2], " task     #175 omp phase 2, your part:") || !strings.HasSuffix(got[3], "…") ||
		got[4] != "          from summer-hamster · 22m ago" || got[5] != "          handed back #183 · 3m ago" ||
		!strings.HasPrefix(got[6], " mail     #190 next: the dsh part · 1m ago") || !strings.HasPrefix(got[7], " team ") {
		t.Fatalf("overview = %q; want the task rows (title over two lines, from, handed back, mail) before team", got)
	}
	if strings.Contains(side("a2"), "task") {
		t.Fatal("a member without an assignment shows a task row")
	}
}

// The footer ends with the daemon's version; another cli build says how to fix it, in full when it
// fits and as the version alone before that; on a terminal too narrow for either the version goes
// and the key hints stay.
func TestTopFooterShowsTheDaemonVersion(t *testing.T) {
	old := Version
	Version = "dev-cli"
	t.Cleanup(func() { Version = old })
	ps := proto.PsResult{Version: "dev-daemon"}
	sgr := regexp.MustCompile("\x1b\\[[0-9;]*m")
	last := func(width int) string {
		m := newTopModel(nil, "")
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		m.Update(fetched{ps: ps})
		lines := strings.Split(sgr.ReplaceAllString(m.render(), ""), "\n")
		return lines[len(lines)-1]
	}
	if l := last(180); !strings.HasSuffix(l, "dev-daemon (cli dev-cli: piggery restart)") {
		t.Fatalf("mismatch footer: %q", l)
	}
	if l := last(80); !strings.HasSuffix(l, "dev-daemon") || strings.Contains(l, "cli") {
		t.Fatalf("mismatch footer, room for the version only: %q", l)
	}
	Version = "dev-daemon"
	if l := last(140); !strings.HasSuffix(l, "dev-daemon") || strings.Contains(l, "cli") {
		t.Fatalf("same version footer: %q", l)
	}
	if l := last(30); strings.Contains(l, "dev-daemon") || strings.TrimSpace(l) == "" {
		t.Fatalf("narrow footer: %q, want the key hints and no version", l)
	}
}

// `x` asks once and `y` kills the selected headless worker (the same verb as `piggery kill`); any
// other key cancels; a session or a stopped worker gets a reason and no question. `x` is also the
// short name of the kill command.
func TestTopKillKey(t *testing.T) {
	ps := proto.PsResult{State: core.State{Teams: []core.TeamState{{ID: "ta", Name: "a", Members: []core.MemberState{
		{ID: "s1", Name: "boss"}, {ID: "w1", Name: "w1", Headless: true, State: "working"}, {ID: "w2", Name: "w2", Headless: true, State: "gone"}}}}}}
	var killedIDs []string
	m := newTopModel(nil, "")
	m.kill = func(id string) error { killedIDs = append(killedIDs, id); return nil }
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	m.Update(fetched{ps: ps})
	press := func(s string) tea.Cmd {
		_, cmd := m.Update(tea.KeyPressMsg{Code: rune(s[0]), Text: s})
		return cmd
	}
	frame := func() string { return regexp.MustCompile("\x1b\\[[0-9;]*m").ReplaceAllString(m.render(), "") }
	m.sel = "w1"
	press("x")
	if !strings.Contains(frame(), "kill w1? y/n") {
		t.Fatalf("no question:\n%s", frame())
	}
	if cmd := press("n"); cmd != nil || len(killedIDs) != 0 || strings.Contains(frame(), "y/n") {
		t.Fatalf("another key: cmd %v, killed %v", cmd != nil, killedIDs)
	}
	press("x")
	cmd := press("y")
	if cmd == nil {
		t.Fatal("y did not kill")
	}
	m.Update(cmd())
	if !slices.Equal(killedIDs, []string{"w1"}) || !strings.Contains(frame(), "killed w1") {
		t.Fatalf("killed %v:\n%s", killedIDs, frame())
	}
	for id, want := range map[string]string{"s1": "boss is not a headless worker: stop it in its own window (Esc)", "w2": "w2 is already stopped"} {
		m.sel = id
		press("x")
		if !strings.Contains(frame(), want) || m.killing.id != "" {
			t.Fatalf("%s: want %q:\n%s", id, want, frame())
		}
	}
	if len(killedIDs) != 1 {
		t.Fatalf("killed %v; only w1", killedIDs)
	}
	if c, _, err := (&env{}).root().Find([]string{"x", "w1"}); err != nil || c.Name() != "kill" {
		t.Fatalf("x resolves to %v, %v; want kill", c, err)
	}
}

// Events start folded to one line of text with the latest event; `e` opens them as a box like the
// Overview's (no column-name line), and again folds them; the keys are two lines with `x kill` on
// the second. An opened list is remembered by the next top; folded is the default, so nothing is saved.
func TestTopEventsBoxAndKeyLines(t *testing.T) {
	sgr := regexp.MustCompile("\x1b\\[[0-9;]*m")
	now := time.Now().UnixMilli()
	ev := func(typ, who string) core.Event { return core.Event{Ts: now - 120_000, Type: typ, Participant: who} }
	m := newTopModel(nil, "")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.Update(fetched{ps: proto.PsResult{State: core.State{Teams: []core.TeamState{{ID: "ta", Name: "a", Members: []core.MemberState{
		{ID: "a1", Name: "a1", Role: "executor", State: "idle"}}}}, Events: []core.Event{ev("spawned", "a1"), ev("handback", "a1")}}}})
	lines := func() []string { return strings.Split(sgr.ReplaceAllString(m.render(), ""), "\n") }

	got := lines()
	if f := got[len(got)-4]; !strings.HasPrefix(f, " ● Events · ") || !strings.Contains(f, "handback") || strings.Contains(strings.Join(got, "\n"), "┌─ Events") {
		t.Fatalf("events start as %q; want folded to the latest event", f)
	}
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	got = lines()
	i := len(got) - 1
	for i >= 0 && !strings.HasPrefix(got[i], "┌─ Events") {
		i--
	}
	if i < 0 || strings.Contains(strings.Join(got, "\n"), "TIME") || !strings.Contains(got[i+1], "handback") {
		t.Fatalf("open events = %q; want a box, newest first, no column names", got[i:])
	}
	keys := got[len(got)-2:]
	if !strings.Contains(keys[0], "enter open/close") || !strings.Contains(keys[1], "x kill") || !strings.HasPrefix(got[len(got)-3], "────") {
		t.Fatalf("key lines = %q; want a rule, then move and look, then act and toggle", got[len(got)-3:])
	}
	// The two key lines are one grid: the i-th entries start at the same cell.
	for _, pair := range [][2]string{{"←/→ tab", "e events"}, {"enter open/close", "m mouse"}, {"esc back", "? all keys"}} {
		if a, b := strings.Index(keys[0], pair[0]), strings.Index(keys[1], pair[1]); lipgloss.Width(keys[0][:a]) != lipgloss.Width(keys[1][:b]) {
			t.Fatalf("key lines = %q; want %q above %q", keys, pair[0], pair[1])
		}
	}

	m.Update(fetched{ps: proto.PsResult{}})
	if got = lines(); !strings.Contains(got[len(got)-5], "No events yet.") {
		t.Fatalf("no events = %q; want the empty-state line", got[len(got)-6:])
	}
	m.Update(fetched{ps: proto.PsResult{State: core.State{Events: []core.Event{ev("handback", "a1")}}}})
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	got = lines()
	folded := got[len(got)-4]
	if !strings.HasPrefix(folded, " ● Events · ") || !strings.Contains(folded, "handback") || strings.Contains(folded, "─") ||
		strings.Contains(strings.Join(got, "\n"), "┌─ Events") {
		t.Fatalf("collapsed events = %q; want one line of text with the latest event, no rule", folded)
	}
}

// A big team: gone members with nobody live below them fold into one line (a gone lead above a
// live worker stays in its place); enter on a team's line folds the whole team to one line with its
// counts, enter on the gone line lists them in the tree, and both choices are remembered by the
// next top (cache/top.json), which drops teams that no longer exist. ps text folds the same.
func TestTopBigTeamsFoldAndRemember(t *testing.T) {
	dir := t.TempDir()
	mem := func(id, state, reports string) core.MemberState {
		return core.MemberState{ID: id, Name: id, State: state, ReportsTo: reports}
	}
	ps := proto.PsResult{State: core.State{Teams: []core.TeamState{{ID: "t", Name: "shop", Members: []core.MemberState{
		mem("boss", "idle", ""), mem("lead", "gone", "boss"), mem("w1", "working", "lead"), mem("w2", "gone", "lead"),
		mem("e1", "gone", "boss"), mem("e2", "gone", "boss"), mem("e3", "idle", "boss")}}}}}
	fresh := func() *topModel {
		m := newTopModel(nil, dir)
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m.Update(fetched{ps: ps})
		return m
	}
	items := func(m *topModel) string { return strings.Join(m.items(), " ") }
	line, folded := closedRow+"t", goneRow+"t"

	m := fresh()
	if got, want := items(m), strings.Join([]string{line, "boss", "lead", "w1", "e3", folded}, " "); got != want {
		t.Fatalf("items %q; want the gone lead kept above its live worker and w2, e1, e2 in one line", got)
	}
	m.sel = folded
	if r := regexp.MustCompile(`▸ 3 members +✗ gone`).FindString(strip(m.render())); r == "" {
		t.Fatalf("the gone row is not drawn as a row of the table (name, state):\n%s", strip(m.render()))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got, want := items(m), strings.Join([]string{line, "boss", "lead", "w1", "w2", "e1", "e2", "e3", folded}, " "); got != want {
		t.Fatalf("with the gone line open items %q; want them in the tree, in order", got)
	}
	if got := items(fresh()); !strings.Contains(got, "e1") {
		t.Fatalf("a new top lists %q; want it to remember the gone members open", got)
	}

	m.sel = line
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := items(m); got != line {
		t.Fatalf("a folded team lists %q; want its line only", got)
	}
	if out := strip(m.render()); !strings.Contains(out, "▸") || !strings.Contains(out, "1 working · 2 idle · 4 gone") {
		t.Fatalf("a folded team is drawn:\n%s", out)
	}
	if got := items(fresh()); got != line {
		t.Fatalf("a new top lists %q; want the team still folded", got)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := items(fresh()); !strings.Contains(got, "boss") {
		t.Fatalf("enter again lists %q; want the team open", got)
	}

	other := proto.PsResult{State: core.State{Teams: []core.TeamState{{ID: "u", Name: "other", Members: []core.MemberState{mem("x", "idle", "")}}}}}
	m.Update(fetched{ps: other})
	m.sel = closedRow + "u"
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	s := loadTopState(dir)
	if v, ok := s.Teams["u"]; len(s.Teams) != 1 || !ok || v || len(s.Gone) != 0 {
		t.Fatalf("saved %+v; want only the team that still exists, folded", s)
	}

	text := psLines(ps, time.Now(), nil, nil)
	var member, gone int
	for _, l := range text {
		switch l.kind {
		case "member":
			member++
		case "gone":
			gone++
			if !strings.Contains(l.text, "3 members") || !strings.Contains(l.text, "✗ gone") {
				t.Fatalf("ps gone line %q", l.text)
			}
		}
	}
	if member != 4 || gone != 1 {
		t.Fatalf("ps lists %d members and %d gone lines; want 4 and 1", member, gone)
	}
}

// strip removes colour codes from a render.
func strip(s string) string { return regexp.MustCompile("\x1b\\[[0-9;]*m").ReplaceAllString(s, "") }

// Opening or closing a fold never moves a column (the widths come from every member, the hidden
// ones too), and a collapsed team line that is longer than the list drops whole counts from the
// end instead of being cut mid-word; a title that does not fit gets an ellipsis.
func TestTopFoldsKeepColumnsAndTeamLineFits(t *testing.T) {
	mem := func(id, state, reports string, h bool) core.MemberState {
		return core.MemberState{ID: id, Name: id, Role: "dev", State: state, ReportsTo: reports, Headless: h, Harness: "pi", Model: "zai/glm-5.3", Cwd: "/w/shop"}
	}
	gate := func(m core.MemberState) core.MemberState { m.Gate = true; return m }
	ps := proto.PsResult{State: core.State{
		Solos: []core.SoloState{{ID: "s1", Name: "s1", State: "idle", Harness: "pi", Cwd: "/w/other"}},
		Teams: []core.TeamState{{ID: "t", Name: "shop-with-a-long-name", Root: "/w/shop", Gate: "boss", Unacked: 3, Members: []core.MemberState{
			gate(mem("boss", "idle", "", false)), mem("w1", "working", "boss", true), mem("w2", "gone", "boss", true)}}}}}
	m := newTopModel(nil, t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(fetched{ps: ps})
	now := time.Now()
	shape := func() [2]string { // the header and the solo row, as drawn
		head, lines, _ := m.list(79, now)
		head = strip(head)
		var solo string
		for _, l := range lines {
			if s := strip(l); strings.Contains(s, "solo  s1") {
				solo = s
			}
		}
		return [2]string{head, solo}
	}
	m.sel = closedRow + "t"
	open := shape()
	if strings.Contains(open[0], "UNACKED") {
		t.Fatalf("top's table has an unacked column: %q", open[0])
	}
	// a team's members start under the team pill's left edge (2 in); a solo has a team line's shape:
	// an empty mark slot, its pill where the team pill is, its name where a team's name is
	_, all, _ := m.list(79, now)
	var boss, soloPill, soloName, teamPill, teamName int
	for _, l := range all {
		s := strip(l)
		cell := func(sub string) int { return lipgloss.Width(s[:strings.Index(s, sub)]) } // cells, not bytes
		switch {
		case strings.Contains(s, "boss"):
			boss = cell("boss")
		case strings.Contains(s, "solo  s1"):
			soloPill, soloName = cell("solo"), cell("s1")
		case strings.Contains(s, "team  shop"):
			teamPill, teamName = cell("team"), cell("shop")
		}
	}
	solo := soloPill - 3 // the NAME column
	if boss != solo+2 || soloPill != teamPill || soloName != teamName {
		t.Fatalf("member at %d, solo pill %d name %d, team pill %d name %d (NAME column %d); want the solo parallel to the team, members 2 in", boss, soloPill, soloName, teamPill, teamName, solo)
	}
	// the directory, a team's mark and a solo share the NAME column; members are one step in
	dir := -1
	for _, l := range all {
		if s := strip(l); strings.HasPrefix(strings.TrimLeft(s, " "), "/w/") {
			dir = len(s) - len(strings.TrimLeft(s, " "))
			break
		}
	}
	if head := strings.Index(open[0], "NAME"); dir != head || solo != head {
		t.Fatalf("directory at %d, NAME at %d, solo at %d; want the directory and the solo under NAME", dir, head, solo)
	}
	// the gate member's row carries the tag, the team line does not
	for _, l := range all {
		s := strip(l)
		if strings.Contains(s, "boss") != strings.Contains(s, "boss (gate)") || strings.Contains(s, "team  shop") && strings.Contains(s, "gate") {
			t.Fatalf("the gate is tagged wrongly: %q", s)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := shape(); got != open {
		t.Fatalf("collapsing the team moved columns:\n%q\n%q", open, got)
	}
	line := func(width int) string {
		_, lines, _ := m.list(width, now)
		for _, l := range lines {
			if s := strip(l); strings.Contains(s, "team  shop") {
				return s
			}
		}
		return ""
	}
	if got := line(79); !strings.Contains(got, "shop-with-a-long-name   1 working · 1 idle · 1 gone") || strings.Contains(got, "gate") {
		t.Fatalf("collapsed team line %q", got)
	}
	for width := 30; width < 100; width++ {
		got := strings.TrimRight(line(width), " ")
		if lipgloss.Width(got) > width-1 {
			t.Fatalf("at %d the line is %d wide: %q", width, lipgloss.Width(got), got)
		}
		title := strings.Index(got, "team")
		if i := strings.Index(got[title:], "   "); i >= 0 { // counts: each whole, never cut
			for _, c := range strings.Split(got[title+i+3:], " · ") {
				if !regexp.MustCompile(`^(\d+ (working|idle|waiting|gone)|unacked \d+)$`).MatchString(c) {
					t.Fatalf("at %d a count is cut: %q in %q", width, c, got)
				}
			}
		}
	}
	if got := line(24); !strings.Contains(got, "…") {
		t.Fatalf("a title that does not fit has no ellipsis: %q", got)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.sel = goneRow + "t"
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := shape(); got != open {
		t.Fatalf("expanding the gone line moved columns:\n%q\n%q", open, got)
	}
}

// scrollPS is a long list: a small team above a team of thirty, in one directory.
func scrollPS() proto.PsResult {
	mem := func(id, reports string) core.MemberState {
		return core.MemberState{ID: id, Name: id, Role: "dev", State: "idle", ReportsTo: reports, Headless: reports != "", Harness: "pi", Cwd: "/w/shop"}
	}
	big := []core.MemberState{mem("boss", "")}
	for i := 1; i < 30; i++ {
		big = append(big, mem(fmt.Sprintf("w%02d", i), "boss"))
	}
	return proto.PsResult{State: core.State{Teams: []core.TeamState{
		{ID: "a", Name: "aaa", Root: "/w/shop", CreatedAt: 1, Members: []core.MemberState{mem("a1", "")}},
		{ID: "b", Name: "big", Root: "/w/shop", CreatedAt: 2, Members: big}}}}
}

// listFrame is the list box of a 120-wide frame as drawn (colours removed), border to border, and the
// row of the terminal it starts on.
func listFrame(m *topModel) ([]string, int) {
	var out []string
	first := -1
	for y, l := range strings.Split(strip(m.render()), "\n") {
		r := []rune(l)
		if len(r) < 80 {
			continue
		}
		if first < 0 && r[0] == '┌' {
			first = y
		}
		if first >= 0 {
			out = append(out, string(r[:80]))
			if r[0] == '└' {
				break
			}
		}
	}
	return out, first
}

// A long list scrolls under a header that stays on the list's first line; the borders say how
// many lines are hidden (↑ above, ↓ below); the window moves only when the selection would leave
// it, so one ↑ after End does not shift it; a click after scrolling selects the row under the
// pointer; and folding a team above the window never leaves blank lines under the last row.
func TestTopScrollsUnderStickyHeader(t *testing.T) {
	m := newTopModel(nil, t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 22})
	m.Update(fetched{ps: scrollPS()})
	mark := func(border, arrow string) string {
		if i := strings.Index(border, arrow); i >= 0 {
			return strings.Fields(border[i:])[0] + " " + strings.Fields(border[i:])[1]
		}
		return ""
	}
	frame := func() (head, up, down string, body []string) {
		f, _ := listFrame(m)
		return f[1], mark(f[0], "↑"), mark(f[len(f)-1], "↓"), f[2 : len(f)-1]
	}
	head, up, down, _ := frame()
	if !strings.Contains(head, "NAME") || up != "" || !strings.HasPrefix(down, "↓ ") {
		t.Fatalf("top: header %q, marks %q %q", head, up, down)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	head, up, down, _ = frame()
	if !strings.Contains(head, "NAME") || !strings.HasPrefix(up, "↑ ") || !strings.HasPrefix(down, "↓ ") {
		t.Fatalf("middle: header %q, marks %q %q", head, up, down)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	head, up, down, body := frame()
	if !strings.Contains(head, "NAME") || !strings.HasPrefix(up, "↑ ") || down != "" || !strings.Contains(body[len(body)-1], "w29") {
		t.Fatalf("bottom: header %q, marks %q %q, last row %q", head, up, down, body[len(body)-1])
	}
	rows := func(b []string) string {
		return strings.Join(mapStrings(b, func(s string) string { r := []rune(s); return string(r[3:]) }), "\n")
	}
	end := rows(body)
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if _, _, _, body = frame(); rows(body) != end {
		t.Fatalf("one ↑ after End moved the window:\n%s\n--\n%s", end, rows(body))
	}
	f, first := listFrame(m)
	var y int
	for i, l := range f {
		if strings.Contains(l, "w20") {
			y = first + i
		}
	}
	m.Update(tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseLeft})
	if m.sel != "w20" {
		t.Fatalf("a click on the w20 row selected %q", m.sel)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	frame()
	for range 3 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	}
	frame()
	m.toggleRow(closedRow + "a") // a fold above the window shortens the list under it
	if _, _, down, body = frame(); down != "" || strings.Trim(body[len(body)-1], "│ ") == "" {
		t.Fatalf("after a fold above the last line is %q, down %q", body[len(body)-1], down)
	}
}

func mapStrings(in []string, f func(string) string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = f(s)
	}
	return out
}

// The m entry of the footer says whether the mouse is on, and the grid does not shift when it
// toggles (both texts have the same width); ? lists it the same way.
func TestTopMouseEntryShowsItsState(t *testing.T) {
	m := newTopModel(nil, t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(fetched{ps: scrollPS()})
	act := func() string { l := strings.Split(strip(m.render()), "\n"); return l[len(l)-1] }
	on := act()
	m.Update(tea.KeyPressMsg{Code: 'm'})
	off := act()
	if !strings.Contains(on, "m mouse on") || !strings.Contains(off, "m mouse off") {
		t.Fatalf("footer %q, then %q; want the state after m mouse", on, off)
	}
	if a, b := strings.Index(on, "? all keys"), strings.Index(off, "? all keys"); lipgloss.Width(on[:a]) != lipgloss.Width(off[:b]) {
		t.Fatalf("the grid shifted:\n%q\n%q", on, off)
	}
	m.Update(tea.KeyPressMsg{Code: '?'})
	if full := strip(m.render()); !strings.Contains(full, "mouse off (off: select text)") {
		t.Fatalf("? does not say the state:\n%s", full)
	}
}

// An event's time is the local clock: seconds today, month-day and minutes on an earlier day.
func TestEventTimeIsLocalClock(t *testing.T) {
	now := time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local)
	at := func(d time.Time) int64 { return d.UnixMilli() }
	if got := eventTime(at(time.Date(2026, 9, 30, 9, 4, 5, 0, time.Local)), now); got != "09:04:05" {
		t.Fatalf("today: %q", got)
	}
	if got := eventTime(at(time.Date(2026, 9, 29, 23, 59, 0, 0, time.Local)), now); got != "09-29 23:59" {
		t.Fatalf("yesterday: %q", got)
	}
}

// An opened events list is remembered by the next top (cache/top.json); folding it again is the
// default, so the choice is forgotten.
func TestTopEventsOpenedSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	if newTopModel(nil, dir).events {
		t.Fatal("events start open; want folded")
	}
	m := newTopModel(nil, dir)
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if !newTopModel(nil, dir).events {
		t.Fatal("an opened events list was not remembered")
	}
	m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if b, _ := os.ReadFile(topStatePath(dir)); strings.Contains(string(b), "events") || newTopModel(nil, dir).events {
		t.Fatalf("folded again, saved %s; want the default, nothing kept", b)
	}
}

// The table's layout depends on the window only, not on the data's states: with nobody working the
// header is the same as with someone working. An all-gone and a closed team are rows on the grid
// (mark, "team", name, "closed" for a closed one; ✗ gone; the time), not a sentence.
func TestTopLayoutIgnoresStatesAndOneLineTeamsAreRows(t *testing.T) {
	now := time.Now().UnixMilli()
	mem := func(id, state string) core.MemberState {
		return core.MemberState{ID: id, Name: id, State: state, Harness: "pi", StateSince: now - 15*3600_000, Cwd: "/w/shop"}
	}
	ps := func(live string) proto.PsResult {
		return proto.PsResult{State: core.State{
			Teams: []core.TeamState{
				{ID: "a", Name: "shop", Root: "/w/shop", CreatedAt: 1, Members: []core.MemberState{mem("a1", live)}},
				{ID: "d", Name: "old", Root: "/w/shop", CreatedAt: 2, Members: []core.MemberState{mem("d1", "gone")}}},
			Closed: []core.ClosedTeam{{TeamState: core.TeamState{ID: "c", Name: "revit", Root: "/w/shop", CreatedAt: 3, Members: []core.MemberState{mem("c1", "gone")}}, ClosedAt: now - 20*60_000}}}}
	}
	view := func(live string) (string, string) {
		m := newTopModel(nil, t.TempDir())
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m.Update(fetched{ps: ps(live)})
		head, lines, _ := m.list(79, time.Now())
		return strip(head), strip(strings.Join(lines, "\n"))
	}
	idle, body := view("idle")
	if working, _ := view("working"); working != idle {
		t.Fatalf("the header follows the states:\n%q\n%q", idle, working)
	}
	for _, want := range []string{`▸  team  old +✗ gone +15h`, `▸  team  revit  closed +✗ gone +20m`} {
		if !regexp.MustCompile(want).MatchString(body) {
			t.Fatalf("no row %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "all gone") || strings.Contains(body, "last active") {
		t.Fatalf("a one-line team is still a sentence:\n%s", body)
	}
}

// When the NAME cell is narrow the gate's name is cut and its tag stays whole.
func TestGateTagSurvivesACutName(t *testing.T) {
	got := strip(row([]string{"  summer-hamster-long (gate)"}, []int{16}, nil, func(int) lipgloss.Style { return lipgloss.NewStyle() }, false, 30))
	if !strings.HasSuffix(strings.TrimRight(got, " "), "… (gate)") || lipgloss.Width(got) != 2+16 {
		t.Fatalf("cell %q; want the name cut to an ellipsis and (gate) whole, 16 wide", got)
	}
}

// `requested` is shown as `queued` (the state itself is unchanged), and CWD is a column only while
// some row of the snapshot has a directory to show, folded and gone rows counted: nothing a fold or a
// state does toggles it.
func TestTopQueuedWordAndCwdOnlyWhenSomeRowHasOne(t *testing.T) {
	if w, _ := stateIcon("requested"); w != "◌ queued" {
		t.Fatalf("requested shows as %q", w)
	}
	mem := func(id, state, cwd string) core.MemberState {
		return core.MemberState{ID: id, Name: id, State: state, ReportsTo: "boss", Headless: true, Harness: "pi", Cwd: cwd}
	}
	boss := core.MemberState{ID: "boss", Name: "boss", State: "idle", Harness: "pi", Cwd: "/w/shop"}
	head := func(gone core.MemberState) string {
		ps := proto.PsResult{State: core.State{Teams: []core.TeamState{{ID: "t", Name: "shop", Root: "/w/shop", Members: []core.MemberState{boss, mem("w1", "working", "/w/shop"), gone}}}}}
		m := newTopModel(nil, t.TempDir())
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		m.Update(fetched{ps: ps})
		h, _, _ := m.list(100, time.Now())
		return strip(h)
	}
	if h := head(mem("w2", "gone", "/w/shop")); strings.Contains(h, "CWD") {
		t.Fatalf("CWD shown with every row in its directory: %q", h)
	}
	if h := head(mem("w2", "gone", "/w/shop/sub")); !strings.Contains(h, "CWD") { // w2 is folded into the gone row
		t.Fatalf("CWD hidden though a (folded) row has one: %q", h)
	}
}
