package view

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sting8k/piggery/internal/core"
)

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
	for _, r := range MemberTree(ms) {
		got = append(got, fmt.Sprintf("%d %s<-%s", r.Depth, r.M.Name, r.M.ReportsTo))
	}
	want := []string{"0 s<-", "1 lead-a<-s", "2 peer-a1<-lead-a", "2 peer-a2<-lead-a", "1 lead-b<-s",
		"2 peer-b1<-lead-b", "0 orphan<-left-the-team"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tree order:\n got %v\nwant %v", got, want)
	}
}
