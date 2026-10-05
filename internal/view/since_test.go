package view

import (
	"testing"
	"time"

	"github.com/sting8k/piggery/internal/core"
)

// An idle row counts from its last real turn: a daemon restart sets a session gone and its reconnect
// idle again, which moves state_since to now but is not a turn. A row that never ran a turn, and a
// working one, keep state_since. The list's "since" and the Overview's agree.
func TestIdleSinceIsTheLastTurn(t *testing.T) {
	now := time.UnixMilli(1_000_000_000_000)
	ago := func(d time.Duration) int64 { return now.Add(-d).UnixMilli() }
	s := core.State{
		Teams: []core.TeamState{{ID: "t", Name: "t", Root: "/p", Members: []core.MemberState{
			{ID: "m", Name: "m", State: "idle", StateSince: ago(23 * time.Minute), LastTurnEnd: ago(5 * time.Hour)},
			{ID: "new", Name: "new", State: "idle", StateSince: ago(23 * time.Minute)},
			{ID: "busy", Name: "busy", State: "working", StateSince: ago(23 * time.Minute), LastTurnEnd: ago(5 * time.Hour)}}}},
		Solos: []core.SoloState{{ID: "s", Name: "s", Cwd: "/q", State: "idle", StateSince: ago(23 * time.Minute), LastTurnEnd: ago(5 * time.Hour)}},
	}
	got := map[string]string{}
	for _, d := range BuildList(ListInput{State: s, Now: now}).Dirs {
		for _, b := range d.Blocks {
			for _, r := range b.Rows {
				got[r.ID] = r.Since
			}
		}
	}
	for id, want := range map[string]string{"m": "5h", "s": "5h", "new": "23m", "busy": "23m"} {
		if got[id] != want {
			t.Errorf("list: %s since %q, want %q", id, got[id], want)
		}
	}
	for id, want := range map[string]string{"m": "5h", "s": "5h"} {
		for _, f := range Describe(s, id, nil, now).Facts {
			if f.Label == "since" && f.Value != want {
				t.Errorf("overview: %s since %q, want %q", id, f.Value, want)
			}
		}
	}
}
