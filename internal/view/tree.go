package view

import "github.com/sting8k/piggery/internal/core"

// TreeRow is a member in tree order: Prefix draws its place under its reports_to (├─ └─ │),
// ParentGone says that parent is gone (top dims it).
type TreeRow struct {
	M          core.MemberState
	Prefix     string
	Depth      int
	ParentGone bool
}

// MemberTree orders a team's members as the tree of reports_to: each parent before its
// children, children in the order they entered the team (the order of ms). A member whose
// parent is not in the team is a root; a gone parent keeps its children.
func MemberTree(ms []core.MemberState) []TreeRow {
	byID := map[string]core.MemberState{}
	for _, m := range ms {
		byID[m.ID] = m
	}
	children := map[string][]core.MemberState{}
	var roots []core.MemberState
	for _, m := range ms {
		if _, ok := byID[m.ReportsTo]; ok && m.ReportsTo != m.ID {
			children[m.ReportsTo] = append(children[m.ReportsTo], m)
		} else {
			roots = append(roots, m)
		}
	}
	var out []TreeRow
	seen := map[string]bool{}
	var walk func(m core.MemberState, indent, branch string, depth int)
	walk = func(m core.MemberState, indent, branch string, depth int) {
		if seen[m.ID] { // reports_to never loops; do not trust that here
			return
		}
		seen[m.ID] = true
		out = append(out, TreeRow{M: m, Prefix: indent + branch, Depth: depth,
			ParentGone: depth > 0 && byID[m.ReportsTo].State == "gone"})
		kids := children[m.ID]
		next := indent
		switch branch {
		case "├─ ":
			next += "│  "
		case "└─ ":
			next += "   "
		}
		for i, k := range kids {
			b := "├─ "
			if i == len(kids)-1 {
				b = "└─ "
			}
			walk(k, next, b, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, "", "", 0)
	}
	for _, m := range ms { // members only reachable through a loop
		if !seen[m.ID] {
			walk(m, "", "", 0)
		}
	}
	return out
}
