package view

import (
	"testing"
	"time"

	"github.com/sting8k/piggery/internal/core"
)

// The Board's columns are titled by the state text of their status's plain state, not by whichever row
// comes first (a waiting column whose first row is queued is still "◐ waiting"), and its directories come with
// their bucket and whether they start folded: live ones open, all-gone and closed ones folded.
func TestBoardColumnsAndFoldedDirs(t *testing.T) {
	now := time.UnixMilli(1_000_000_000_000)
	ago := func(d time.Duration) int64 { return now.Add(-d).UnixMilli() }
	s := core.State{
		Teams: []core.TeamState{
			{ID: "live", Name: "live", Root: "/live", CreatedAt: ago(time.Hour), Members: []core.MemberState{
				{ID: "q", Name: "q", State: "requested"}, {ID: "p", Name: "p", State: "awaiting_permission"}}},
			{ID: "dead", Name: "dead", Root: "/dead", CreatedAt: ago(2 * time.Hour), Members: []core.MemberState{{ID: "g", Name: "g", State: "gone"}}}},
		Closed: []core.ClosedTeam{{TeamState: core.TeamState{ID: "c", Name: "c", Root: "/old", CreatedAt: ago(100 * time.Hour)}, ClosedAt: ago(50 * time.Hour)}},
	}
	b := Snapshot(s, SnapshotInput{Now: now}).Board
	words := map[Status]string{}
	for _, c := range b.Columns {
		words[c.Status] = c.Word
	}
	if words[StatusWaiting] != "◐ waiting" || words[StatusWorking] != "● working" || words[StatusIdle] != "○ idle" || words[StatusGone] != "✗ gone" {
		t.Errorf("column words %v", words)
	}
	// the Board says a card's state only when its state_text is not its column's: a plain waiting
	// row's is the column's, a queued one's is not
	for _, d := range b.Dirs {
		for _, bl := range d.Blocks {
			for _, r := range bl.Rows {
				if r.Kind == KindMember && r.ID == "p" && r.StateText != words[r.Status] {
					t.Errorf("a plain waiting row says %q, its column %q", r.StateText, words[r.Status])
				}
				if r.Kind == KindMember && r.ID == "q" && r.StateText == words[r.Status] {
					t.Errorf("a queued row says %q, the same as its column", r.StateText)
				}
			}
		}
	}
	got := map[string]string{}
	for _, d := range b.Dirs {
		got[d.Path] = d.Bucket
		if d.Folded != (d.Bucket == "gone" || d.Bucket == "closed") {
			t.Errorf("%s: bucket %s, folded %v", d.Path, d.Bucket, d.Folded)
		}
	}
	if len(b.Dirs) != 3 || got["/live"] != "live" || got["/dead"] != "gone" || got["/old"] != "closed" {
		t.Errorf("board dirs %v; want /live live, /dead gone and the closed /old, in one list", got)
	}
}
