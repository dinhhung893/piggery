package cli

import (
	"cmp"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"

	"github.com/sting8k/piggery/internal/core"
	"github.com/sting8k/piggery/internal/proto"
	"github.com/sting8k/piggery/internal/server"
)

// The look of top: gh-dash's master-detail
// (status header, tabs, list and sidebar boxes, events strip, key footer with ? for all) in
// Charm's charmtone palette, as fang's help and errors (fang DefaultColorScheme). Rows carry
// icon + coloured text only; pills are for the header and the sidebar. Every width comes from
// fitCols and box.

// The palette, set by usePalette for a light or dark terminal background. Plain text keeps the
// terminal's own foreground.
var (
	colPrimary, colSuccess, colMuted, colWarning, colError, colSubtle, colSurface, colTool color.Color
	colInk                                                                                 color.Color // text on a coloured pill
	colOnMain                                                                              color.Color // text on the primary pill

	stTitle, stMuted, stRule lipgloss.Style
	stPlain                  = lipgloss.NewStyle()
)

func init() { usePalette(true) }

// usePalette sets top's colours for a dark or light background: the one place they are chosen.
// Pairs are (light, dark) as in fang; charmtone names as Crush and fang use them.
func usePalette(dark bool) {
	c := lipgloss.LightDark(dark)
	colPrimary = charmtone.Charple
	colSuccess = c(lipgloss.Color("#0CB37F"), charmtone.Guac) // fang's flag green
	colMuted = charmtone.Squid
	colWarning = c(charmtone.Tang, charmtone.Mustard) // Mustard is unreadable on a light background
	colError = c(charmtone.Sriracha, charmtone.Cherry)
	colSubtle = c(charmtone.Smoke, charmtone.Oyster)
	colSurface = c(charmtone.Ash, charmtone.Charcoal)
	colTool = charmtone.Dolly
	colInk = charmtone.Pepper
	colOnMain = charmtone.Butter
	stTitle = lipgloss.NewStyle().Bold(true).Foreground(colPrimary)
	stMuted = lipgloss.NewStyle().Foreground(colMuted)
	stRule = lipgloss.NewStyle().Foreground(colSubtle)
}

// helpStyles is the key footer in the palette: keys muted, descriptions and separators subtle.
func helpStyles() help.Styles {
	return help.Styles{ShortKey: stMuted, ShortDesc: stRule, ShortSeparator: stRule, Ellipsis: stRule,
		FullKey: stMuted, FullDesc: stRule, FullSeparator: stRule}
}

// stateIcon is a member state as icon + word, coloured by what it asks of the operator.
func stateIcon(state string) (string, color.Color) {
	switch state {
	case "working":
		return "● working", colSuccess
	case "idle":
		return "○ idle", colMuted
	case "requested":
		return "◌ queued", colWarning // the state is `requested`; top says what it is for a person
	case "starting":
		return "◌ starting", colWarning
	case "awaiting_permission":
		return "◐ waiting", colWarning
	case "parked":
		return "⏸ parked", colWarning
	case "gone":
		return "✗ gone", colError
	}
	return state, colMuted
}

// allStates are the states a participant can be in, for the width of the STATE column (which sizes
// to the widest word stateIcon shows for them, `requested` being shown as `queued`).
var allStates = []string{"requested", "starting", "working", "idle", "awaiting_permission", "parked", "gone"}

// gateTag follows the name of a team's gate member in its NAME cell, drawn muted by row(); the
// cell's width includes it, so nothing moves.
const gateTag = " (gate)"

func gateTagOf(m core.MemberState) string {
	if m.Gate {
		return gateTag
	}
	return ""
}

// A solo's name cell has a team line's shape: the fold-mark slot, empty (a solo has nothing to
// fold), a pill of the team pill's width in a quiet colour, then the name. Plain text with the
// pill drawn by row(), so widths and truncation see cells only.
const (
	soloSlot = "  "
	soloPill = " solo "
	soloLead = soloSlot + soloPill + " "
)

// dirLabel is a directory line's text: a path, so it ends in "/".
func dirLabel(p string) string {
	return strings.TrimSuffix(p, "/") + "/"
}

func pill(text string, bg, fg color.Color) string {
	return lipgloss.NewStyle().Background(bg).Foreground(fg).Padding(0, 1).Render(text)
}

// topColumns are the columns top's table can show: every configurable one but unacked, which top
// gives in its header, the team lines and a member's details; ps text keeps its unacked= field.
var topColumns = slices.DeleteFunc(slices.Clone(server.DisplayColumns), func(c string) bool { return c == "unacked" })

// anyCwd reports whether any member or solo of the snapshot, folded, gone or closed ones included and
// whatever the tab, works outside its group's directory: the rows CWD would show something for.
func (m *topModel) anyCwd() bool {
	var roots []string
	for _, t := range teamsOf(m.ps) {
		roots = append(roots, t.Root)
	}
	for _, g := range groupByDir(m.ps.Teams, m.ps.Closed, m.ps.Solos, roots) {
		for _, u := range g.units {
			if u.solo != nil && relCwd(g.dir, u.solo.Cwd) != "" {
				return true
			}
			if u.team != nil {
				for _, mem := range u.team.Members {
					if relCwd(g.dir, mem.Cwd) != "" {
						return true
					}
				}
			}
		}
	}
	return false
}

// When the list is narrow: CWD and MODEL shrink, then ROLE, MODEL, TURNS and AGE go, then NAME
// shrinks.
var (
	listFit = []namedFit{{"cwd", 8}, {"model", 8}, {"role", 0}, {"model", 0}, {"turns", 0}, {"age", 0}, {"name", 8}}

	eventCols = []string{"TIME", "WHO", "EVENT", "TARGET"}
	eventFit  = []fitStep{{3, 6}, {1, 6}, {2, 8}}
)

const (
	gap       = 2  // cells between columns
	sideMin   = 40 // the sidebar is at least this wide ...
	sideRight = 110
	// ... and goes right of the list from this terminal width, below it under.
)

// fitStep shrinks col to min cells when the row is too wide; min 0 hides the column.
type fitStep struct{ col, min int }

// fitCols sizes each column to its widest cell (header included), then applies steps in order
// until the row fits width. A width of 0 hides a column.
func fitCols(head []string, rows [][]string, width int, steps []fitStep) []int {
	w := make([]int, len(head))
	for c, h := range head {
		w[c] = lipgloss.Width(h)
	}
	for _, r := range rows {
		for c, v := range r {
			w[c] = max(w[c], lipgloss.Width(v))
		}
	}
	total := func() int {
		n := -gap
		for _, x := range w {
			if x > 0 {
				n += x + gap
			}
		}
		return n
	}
	for _, s := range steps {
		over := total() - width
		if over <= 0 {
			break
		}
		if s.min == 0 {
			w[s.col] = 0
		} else {
			w[s.col] = max(s.min, w[s.col]-over)
		}
	}
	return w
}

// row renders cells in widths after a 2-cell selection bar; a selected row is filled to width
// with the surface colour (gh-dash), others are not.
func row(cells []string, widths []int, right map[int]bool, style func(c int) lipgloss.Style, sel bool, width int) string {
	bg := func(s lipgloss.Style) lipgloss.Style {
		if sel {
			return s.Background(colSurface)
		}
		return s
	}
	var b strings.Builder
	bar := "  "
	if sel {
		bar = "▌ "
	}
	b.WriteString(bg(lipgloss.NewStyle().Foreground(colPrimary)).Render(bar))
	n, first := 2, true
	for c, v := range cells {
		if widths[c] == 0 {
			continue
		}
		if !first {
			b.WriteString(bg(stPlain).Render(strings.Repeat(" ", gap)))
			n += gap
		}
		first = false
		tag := ""
		if c == 0 && strings.HasSuffix(v, gateTag) { // the gate's tag stays whole when the name is cut
			v, tag = strings.TrimSuffix(v, gateTag), gateTag
		}
		v = truncate(v, widths[c]-lipgloss.Width(tag))
		fill := strings.Repeat(" ", widths[c]-lipgloss.Width(v)-lipgloss.Width(tag))
		if right[c] {
			v = fill + v
		} else if tag == "" {
			v += fill
		}
		if tag != "" {
			b.WriteString(bg(style(c)).Render(v))
			b.WriteString(bg(stMuted).Render(tag))
			b.WriteString(bg(style(c)).Render(fill))
		} else if c == 0 && strings.HasPrefix(v, soloLead) { // a solo's pill, quiet, where a team has its own
			rest := strings.TrimPrefix(v, soloLead)
			b.WriteString(bg(style(c)).Render(soloSlot))
			b.WriteString(bg(lipgloss.NewStyle().Foreground(colMuted).Background(colSurface)).Render(soloPill))
			b.WriteString(bg(style(c)).Render(" " + rest))
		} else {
			b.WriteString(bg(style(c)).Render(v))
		}
		n += widths[c]
	}
	if sel && n < width {
		b.WriteString(bg(stPlain).Render(strings.Repeat(" ", width-n)))
	}
	return b.String()
}

// box draws lines in a w x h border with an optional title in the top edge; lines are cut or
// padded to fit.
func box(title string, lines []string, w, h int) []string {
	return boxMarked(title, lines, w, h, "", "")
}

// boxMarked is box with a muted note at the right end of the top and of the bottom border ("" for
// none): what is hidden above and below.
func boxMarked(title string, lines []string, w, h int, above, below string) []string {
	inner := max(w-2, 0)
	// edge draws a border of dashes from a left part of left cells to the corner, the note before it
	edge := func(l, r, left string, note string) string {
		room := inner - (lipgloss.Width(l) - 1) - lipgloss.Width(left) // dashes the border has room for
		if note != "" && room >= lipgloss.Width(note)+4 {
			note = " " + note + " "
			return stRule.Render(l) + left + stRule.Render(strings.Repeat("─", room-lipgloss.Width(note)-1)) + stMuted.Render(note) + stRule.Render("─"+r)
		}
		return stRule.Render(l) + left + stRule.Render(strings.Repeat("─", max(room, 0))+r)
	}
	top := edge("┌", "┐", "", above)
	if title != "" {
		t := lipgloss.NewStyle().MaxWidth(max(inner-2, 0)).Render(" " + title + " ")
		top = stRule.Render("┌─") + t + stRule.Render(strings.Repeat("─", max(inner-1-lipgloss.Width(t), 0))+"┐")
		if above != "" {
			top = edge("┌─", "┐", t, above)
		}
	}
	out := []string{top}
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lipgloss.NewStyle().MaxWidth(inner).Render(lines[i])
		}
		out = append(out, stRule.Render("│")+l+strings.Repeat(" ", max(inner-lipgloss.Width(l), 0))+stRule.Render("│"))
	}
	return append(out, edge("└", "┘", "", below))
}

func (m *topModel) render() string {
	if !m.loaded {
		return "loading…"
	}
	now := time.Now()
	width, height := m.w, m.h
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 30
	}
	cut := lipgloss.NewStyle().MaxWidth(width)

	m.hits = m.hits[:0]
	head := []string{cut.Render(m.header(now)), stRule.Render(strings.Repeat("─", width))}
	if tabs := m.tabs(); len(tabs) > 1 {
		head = append(head, cut.Render(m.tabBar(tabs, len(head))))
	}
	foot := m.eventsBox(width, height, now)
	if m.killing.id != "" { // the one question, above the keys
		foot = append(foot, " "+lipgloss.NewStyle().Foreground(colWarning).Bold(true).Render("kill "+m.killing.name+"? y/n"))
	} else if m.killNote != "" {
		foot = append(foot, " "+lipgloss.NewStyle().Foreground(colWarning).Render(truncate(m.killNote, width-2)))
	}
	m.help.SetWidth(width - 2)
	foot = append(foot, stRule.Render(strings.Repeat("─", width))) // mirrors the rule under the header
	for _, l := range m.keyLines(width - 2) {
		foot = append(foot, " "+l)
	}
	notes, mismatch := versionNotes(m.ps.Version, Version)
	foot[len(foot)-1] = withVersion(foot[len(foot)-1], notes, mismatch, width)

	room := max(height-len(head)-len(foot), 6)
	y := len(head)
	var main []string
	switch {
	case !m.sideShown() || m.sel == "": // no sidebar with nothing to show in it
		main = m.listBox(width, room, y, now)
	case !m.narrow():
		sw := max(sideMin, width/3)
		left := m.listBox(width-sw, room, y, now)
		right := m.sideBox(width-sw, y, sw, room, now)
		for i := range left {
			main = append(main, left[i]+right[i])
		}
	default: // narrow: the sidebar instead of the list, until esc
		main = m.sideBox(0, y, width, room, now)
	}
	return strings.Join(append(append(head, main...), foot...), "\n")
}

// hit is a clickable span of one screen row, recorded while drawing: a tab (tab), a row of
// the list (id), a sidebar tab (side >= 0), or the list's body (list: the wheel moves there).
type hit struct {
	y, x0, x1 int
	tab, id   string
	side      int
	list      bool
}

func (m *topModel) at(x, y int) (hit, bool) {
	for _, h := range m.hits {
		if h.y == y && x >= h.x0 && x < h.x1 {
			return h, true
		}
	}
	return hit{}, false
}

// listBox is the list in a w x h box at row y (left edge 0): the column header on its first line,
// then the body from m.scroll, which moves only when the selection would leave the body (with a
// line of margin). When lines are hidden the borders say how many (↑ N above, ↓ N below). It
// records a hit per drawn line.
func (m *topModel) listBox(w, h, y int, now time.Time) []string {
	head, body, ids := m.list(w-2, now)
	n := max(h-2, 0) // lines in the box
	room := n
	if head != "" {
		room--
	}
	m.page = max(room, 1)
	m.scroll = min(max(m.scroll, 0), max(len(body)-room, 0)) // the list shrank: no blank space under the last line
	if sel := slices.Index(ids, m.sel); sel >= 0 && room > 0 {
		if sel < m.scroll+1 {
			m.scroll = max(sel-1, 0)
		} else if sel > m.scroll+room-2 {
			m.scroll = min(sel-room+2, max(len(body)-room, 0))
		}
	}
	start := m.scroll
	shown, shownIDs := body[min(start, len(body)):min(start+max(room, 0), len(body))], ids[min(start, len(ids)):min(start+max(room, 0), len(ids))]
	above, below := start, max(len(body)-start-len(shown), 0)
	lines, rowIDs := shown, shownIDs
	if head != "" {
		lines, rowIDs = append([]string{head}, shown...), append([]string{""}, shownIDs...)
	}
	for i := range n {
		hl := hit{y: y + 1 + i, x0: 1, x1: w - 1, side: -1, list: true}
		if i < len(rowIDs) {
			hl.id = rowIDs[i]
		}
		m.hits = append(m.hits, hl)
	}
	var up, down string
	if above > 0 {
		up = fmt.Sprintf("↑ %d", above)
	}
	if below > 0 {
		down = fmt.Sprintf("↓ %d", below)
	}
	return boxMarked("", lines, w, h, up, down)
}

// sideBox is the sidebar in a w x h box at column x, row y; its Overview and Tail titles are
// hits.
func (m *topModel) sideBox(x, y, w, h int, now time.Time) []string {
	ox := x + 3 // after "┌─ "
	m.hits = append(m.hits, hit{y: y, x0: ox, x1: ox + len("Overview"), side: sideOverview},
		hit{y: y, x0: ox + len("Overview  "), x1: ox + len("Overview  Tail"), side: sideTail})
	return box(m.sideTitle(), m.sidebar(w-2, h-2, now), w, h)
}

// header is the title pill and the daemon's status: counts muted, held and unacked as pills
// (amber when mail is held).
func (m *topModel) header(now time.Time) string {
	working, idle := 0, 0
	count := func(state string) {
		switch state {
		case "working":
			working++
		case "idle":
			idle++
		}
	}
	for _, t := range m.ps.Teams {
		for _, mem := range t.Members {
			count(mem.State)
		}
	}
	for _, s := range m.ps.Solos {
		count(s.State)
	}
	held := pill(fmt.Sprintf("held %d", m.ps.Held), colSurface, colMuted)
	if m.ps.Held > 0 {
		held = pill(fmt.Sprintf("held %d", m.ps.Held), colWarning, colInk)
	}
	head := " " + lipgloss.NewStyle().Bold(true).Render(pill("🐷 piggery", colPrimary, colOnMain)) + " " +
		pill("● daemon "+ago(m.ps.StartedAt, now), colSurface, colSuccess) + " " +
		stMuted.Render(fmt.Sprintf(" %s · %d working · %d idle ", plural(len(m.ps.Teams), "team"), working, idle)) +
		held + " " + pill(fmt.Sprintf("unacked %d", m.ps.Unacked), colSurface, colMuted)
	if n := proto.Notice(m.ps.Outdated); n != "" { // an install only `piggery setup --outdated` brings up to date
		head += " " + pill(n, colWarning, colInk)
	}
	return head
}

func (m *topModel) tabBar(tabs []topTab, y int) string {
	var parts []string
	x := 2
	for _, t := range tabs {
		label := fmt.Sprintf("%s %d", t.label, t.count)
		w := lipgloss.Width(label)
		m.hits = append(m.hits, hit{y: y, x0: x, x1: x + w, tab: t.key, side: -1})
		x += w + 3
		if t.key == m.tab {
			parts = append(parts, stTitle.Underline(true).Render(label))
		} else {
			parts = append(parts, stMuted.Render(label))
		}
	}
	return "  " + strings.Join(parts, "   ")
}

// list is the column header ("" when there are no rows) and the current tab's lines, by project directory (groupByDir): the directory, then its
// units oldest first (a team's title and its members' tree; a closed team's line, its members
// when expanded; a solo's row), all rows in one table with the same columns. Per line, the id of
// the member, solo or closed team on it ("" for other lines).
func (m *topModel) list(width int, now time.Time) (string, []string, []string) {
	stats := m.stats()
	var lines, ids []string
	// line appends l, for id when it is a selectable row.
	line := func(l, id string) {
		lines, ids = append(lines, l), append(ids, id)
	}
	muted := func(s string) string { return "  " + stMuted.Render(s) }

	kinds := topColumns
	if !m.anyCwd() { // CWD only when some row has one to show, decided from every row so a fold or a state never toggles it
		kinds = slices.DeleteFunc(slices.Clone(topColumns), func(c string) bool { return c == "cwd" })
	}
	cols := shown(m.cols, kinds)
	head, right, fit := layout(cols, listFit)
	cellsOf := func(val map[string]string) []string {
		out := make([]string, len(cols))
		for c, name := range cols {
			out[c] = cmp.Or(val[name], "-")
		}
		return out
	}
	groups := m.groups()
	dirs := make([]string, len(groups))
	for i, g := range groups {
		dirs[i] = g.dir
	}
	short := shortPaths(dirs)

	// The rows first (their widths are shared by the whole table), then the lines in order.
	type entry struct {
		text string // a line that is not a row ("" = the row)
		id   string
		row  *trow
	}
	var entries []entry
	var all [][]string
	addRow := func(r trow) {
		entries = append(entries, entry{row: &r})
		all = append(all, r.cells)
	}
	// usage is id's ctx and turns as cells, "-" when no log of it was read.
	usage := func(id string) (ctx, turns string) {
		ctx, turns = "-", "-"
		if ws, ok := stats[id]; ok {
			if ws.hasCtx {
				ctx = tokens(ws.ctx)
			}
			turns = fmt.Sprint(ws.turns)
		}
		return ctx, turns
	}
	memberRow := func(g dirGroup, tr treeRow, closed bool) trow {
		mem := tr.m
		state, _ := stateIcon(mem.State)
		ctx, turns := usage(mem.ID)
		cwd := relCwd(g.dir, mem.Cwd)
		return trow{id: mem.ID, state: mem.State, dim: closed || mem.State == "gone" || tr.parentGone, cells: cellsOf(map[string]string{
			"name": memberIndent + tr.prefix + mem.Name + gateTagOf(mem), "role": mem.Role, "state": state, "harness": harnessLabel(mem.Harness, mem.Headless),
			"model": modelID(mem.Model), "ctx": ctx, "turns": turns, "unacked": fmt.Sprint(mem.Unacked),
			"age": ago(mem.CreatedAt, now), "since": ago(mem.StateSince, now), "cwd": cmp.Or(cwd, " ")})}
	}
	member := func(g dirGroup, tr treeRow, closed bool) { addRow(memberRow(g, tr, closed)) }
	// Opening or closing a fold never moves a column: the widths come from every member of every
	// team, the ones a fold hides included.
	var hidden [][]string
	sizeOf := func(g dirGroup, t *core.TeamState) {
		for _, tr := range memberTree(t.Members) {
			hidden = append(hidden, memberRow(g, tr, false).cells)
		}
		if _, gone := foldGone(t.Members); len(gone) > 0 && !dead(*t) {
			hidden = append(hidden, goneMembersRow(teamView{gone: gone}, t.ID, cols, now).cells)
		}
	}
	for gi, g := range groups {
		if gi > 0 {
			entries = append(entries, entry{text: " "})
		}
		entries = append(entries, entry{text: "  " + stMuted.Render(dirLabel(short[gi]))}) // under the NAME header
		for _, u := range g.units {
			switch {
			case u.solo != nil:
				s := u.solo
				state, _ := stateIcon(s.State)
				ctx, turns := usage(s.ID)
				addRow(trow{id: s.ID, state: s.State, cells: cellsOf(map[string]string{
					"name": soloLead + s.Name, "state": state, "harness": harnessLabel(s.Harness, false),
					"model": modelID(s.Model), "ctx": ctx, "turns": turns, "unacked": fmt.Sprint(s.Unacked), "age": ago(s.CreatedAt, now),
					"since": ago(s.StateSince, now), "cwd": cmp.Or(relCwd(g.dir, s.Cwd), " ")})})
			case m.oneLine(u):
				t := u.team
				sizeOf(g, t)
				mark := "▸ "
				if m.teamOpen(t.ID, false) {
					mark = "▾ "
				}
				at, word := u.active(), ""
				if c := u.closed; c != nil {
					at, word = c.ClosedAt, "closed"
				}
				addRow(oneLineTeamRow(t.ID, t.Name, mark, word, at, cols, now))
				if m.teamOpen(t.ID, false) {
					for _, tr := range memberTree(t.Members) {
						member(g, tr, u.closed != nil)
					}
				}
			default:
				t := u.team
				sizeOf(g, t)
				open := m.tab != "" || m.teamOpen(t.ID, true)
				if m.tab == "" { // All: the team's line can fold it; in its own tab the title is only a title
					entries = append(entries, entry{id: closedRow + t.ID, text: m.teamLine(*t, open, m.sel == closedRow+t.ID, width)})
				} else {
					entries = append(entries, entry{text: stepIndent + stepIndent + m.teamTitle(*t, func(s lipgloss.Style) lipgloss.Style { return s }, width-6)})
				}
				if !open {
					continue
				}
				if len(t.Members) == 0 {
					entries = append(entries, entry{text: muted(stepIndent + "No members. Open an agent session in " + home(t.Root) + " and ask the gate to admit it.")})
					continue
				}
				v := m.membersOf(*t)
				for _, tr := range v.rows {
					member(g, tr, false)
				}
				if len(v.gone) > 0 {
					addRow(goneMembersRow(v, t.ID, cols, now))
				}
			}
		}
	}
	if c := slices.Index(cols, "state"); c >= 0 { // the widest state word top can show: STATE's width never follows the current states
		phantom := make([]string, len(cols))
		for _, s := range allStates {
			if w, _ := stateIcon(s); lipgloss.Width(w) > lipgloss.Width(phantom[c]) {
				phantom[c] = w
			}
		}
		hidden = append(hidden, phantom)
	}
	widths := fitCols(head, append(all[:len(all):len(all)], hidden...), width-3, fit)
	if c := slices.Index(cols, "cwd"); c >= 0 { // a path too wide keeps its end
		for _, r := range all {
			r[c] = truncLeft(r[c], widths[c])
		}
	}
	var header string // one header: the table is shared by every directory; the box keeps it on its first line
	if len(all) > 0 {
		header = row(head, widths, right, func(int) lipgloss.Style { return stMuted }, false, width)
	}
	for _, e := range entries {
		switch {
		case e.row != nil:
			r := e.row
			line(row(r.cells, widths, right, func(c int) lipgloss.Style {
				switch {
				case r.dim:
					return lipgloss.NewStyle().Foreground(colSubtle)
				case cols[c] == "state":
					_, col := stateIcon(r.state)
					return lipgloss.NewStyle().Foreground(col)
				case cols[c] == "name":
					return stPlain
				}
				return stMuted
			}, r.id == m.sel, width), r.id)
		default:
			line(e.text, e.id)
		}
	}
	if len(groups) == 0 {
		switch m.tab {
		case "":
			line(muted("No teams. In pi: “found a team here”"), "")
		case tabClosed:
			line(muted("No closed teams."), "")
		}
	}
	return header, lines, ids
}

// teamTitle is the pill, the name, `no gate` when it has none, and the held count; in room cells (0: no limit), the
// part that does not fit ends in an ellipsis and what follows it is left out.
func (m *topModel) teamTitle(t core.TeamState, bg func(lipgloss.Style) lipgloss.Style, room int) string {
	warn := lipgloss.NewStyle().Foreground(colWarning)
	type seg struct {
		text  string
		style lipgloss.Style
	}
	segs := []seg{{t.Name, stTitle}} // the gate is tagged on its member's row, not here
	if t.Gate == "" {
		segs = append(segs, seg{" · no gate", warn})
	}
	if t.Held > 0 {
		segs = append(segs, seg{fmt.Sprintf(" · %d held", t.Held), warn})
	}
	// a pill tells it from the directory line above, which is its root
	title := pill("team", colPrimary, colOnMain) + bg(stPlain).Render(" ")
	limit := room > 0
	room -= lipgloss.Width(title)
	for _, s := range segs {
		text := s.text
		if limit {
			text = truncate(text, room)
			room -= lipgloss.Width(text)
		}
		title += bg(s.style).Render(text)
	}
	return title
}

// teamLine is a live team's line in All: a selection bar and a ▾/▸ mark, then its title (gate, held
// amber); folded, also the counts of its members by state and its unacked mail. A selected line
// is filled to width like a row.
func (m *topModel) teamLine(t core.TeamState, open, sel bool, width int) string {
	bg := func(s lipgloss.Style) lipgloss.Style {
		if sel {
			return s.Background(colSurface)
		}
		return s
	}
	bar, mark := "  ", "▸ " // the pill's own padding is on its background, so a space sets it off
	if sel {
		bar = "▌ "
	}
	if open {
		mark = "▾ "
	}
	room := width - 1 - 4 // as a row: the bar and the mark, and a cell of margin
	line := bg(lipgloss.NewStyle().Foreground(colPrimary)).Render(bar) + bg(stMuted).Render(mark) + m.teamTitle(t, bg, room)
	if !open {
		var working, idle, other, gone int
		for _, mem := range t.Members {
			switch mem.State {
			case "working":
				working++
			case "idle":
				idle++
			case "gone":
				gone++
			default:
				other++
			}
		}
		var parts []string
		for _, c := range []struct {
			n    int
			what string
		}{{working, "%d working"}, {idle, "%d idle"}, {other, "%d waiting"}, {gone, "%d gone"}, {t.Unacked, "unacked %d"}} { // held is in the title, amber
			if c.n > 0 {
				parts = append(parts, fmt.Sprintf(c.what, c.n))
			}
		}
		if len(parts) == 0 {
			parts = []string{"no members"}
		}
		// the counts that fit, whole, the last ones dropped first; none at all if the title takes the room
		left := room - lipgloss.Width(m.teamTitle(t, func(s lipgloss.Style) lipgloss.Style { return s }, 0)) - 3
		for len(parts) > 0 && lipgloss.Width(strings.Join(parts, " · ")) > left {
			parts = parts[:len(parts)-1]
		}
		if len(parts) > 0 {
			line += bg(stMuted).Render("   " + strings.Join(parts, " · "))
		}
	}
	if pad := width - lipgloss.Width(line); sel && pad > 0 {
		line += bg(stPlain).Render(strings.Repeat(" ", pad))
	}
	return line
}

// The list's levels, in cells of the NAME cell (which starts under the NAME header): the directory
// line, a team's mark, a closed team's line and a solo at 0; a team's members (their tree, and the
// gone row) one step in, under the team pill's left edge. The indent is part of the NAME cell, so
// no column moves.
const (
	stepIndent   = "  "
	memberIndent = stepIndent
)

// oneLineTeamRow is the row of a team listed as one line (all its members gone, or closed), on the
// table's columns: the mark, the team as the live team's line has it, its name and "closed" for a
// closed one in NAME; a gone member's state; in SINCE when it was last active (or closed). Dim, the
// rest empty; its members, when open, follow as rows.
func oneLineTeamRow(id, name, mark, closed string, at int64, cols []string, now time.Time) trow {
	nameCell := mark + " team  " + name // " team " is the pill's width and padding; here plain, dim
	if closed != "" {
		nameCell += "  " + closed
	}
	state, _ := stateIcon("gone")
	val := map[string]string{"name": nameCell, "state": state, "since": ago(at, now)}
	cells := make([]string, len(cols))
	for c, col := range cols {
		cells[c] = val[col]
	}
	return trow{id: closedRow + id, state: "gone", dim: true, cells: cells}
}

// trow is one row of the list's table: its cells, and what the row is (a member, a solo, a team's
// folded gone members) for its selection and colours.
type trow struct {
	cells []string
	id    string
	state string
	dim   bool
}

// goneMembersRow is the row of a team's folded gone members, on the table's columns like a member
// row: ▸ (▾ open) and how many in NAME, a gone member's state, and in SINCE when the latest went
// gone; every other cell is empty. Dim like the gone rows it stands for.
func goneMembersRow(v teamView, teamID string, cols []string, now time.Time) trow {
	mark := "▸ "
	if v.open {
		mark = "▾ "
	}
	var last int64
	for _, g := range v.gone {
		last = max(last, g.StateSince)
	}
	state, _ := stateIcon("gone")
	val := map[string]string{"name": memberIndent + mark + plural(len(v.gone), "member"), "state": state, "since": ago(last, now)}
	cells := make([]string, len(cols))
	for c, name := range cols {
		cells[c] = val[name]
	}
	return trow{id: goneRow + teamID, state: "gone", dim: true, cells: cells}
}

func (m *topModel) sideTitle() string {
	tabs := []string{"Overview", "Tail"}
	for i, t := range tabs {
		if i == m.sideTab {
			tabs[i] = stTitle.Underline(true).Render(t)
		} else {
			tabs[i] = stMuted.Render(t)
		}
	}
	return strings.Join(tabs, "  ")
}

// sidebar is the selected row's Overview (every fact top has of it) or its Tail, in w x h.
func (m *topModel) sidebar(w, h int, now time.Time) []string {
	var team *core.TeamState
	var mem *core.MemberState
	var solo *core.SoloState
	teams := make([]*core.TeamState, 0, len(m.ps.Teams)+len(m.ps.Closed))
	for i := range m.ps.Teams {
		teams = append(teams, &m.ps.Teams[i])
	}
	kv := func(k, v string) string { return " " + stMuted.Render(fmt.Sprintf("%-9s", k)) + v }
	for i := range m.ps.Teams {
		t := &m.ps.Teams[i]
		if m.sel == goneRow+t.ID { // the line of its folded gone members
			_, gone := foldGone(t.Members)
			out := []string{" " + lipgloss.NewStyle().Bold(true).Render(fmt.Sprint(len(gone), " gone")) + stMuted.Render(" · "+t.Name), ""}
			for _, g := range gone {
				out = append(out, kv(g.Role, g.Name+"  "+stMuted.Render(ago(g.StateSince, now)+" ago")))
			}
			return append(out, "", " "+stMuted.Render("enter lists them in the tree"))
		}
		if m.sel == closedRow+t.ID && !dead(*t) { // a live team's line: the team's facts
			var working, idle, gone int
			for _, mem := range t.Members {
				switch mem.State {
				case "working":
					working++
				case "idle":
					idle++
				case "gone":
					gone++
				}
			}
			out := []string{" " + lipgloss.NewStyle().Bold(true).Render(t.Name) + stMuted.Render(" · open"), "", kv("members", fmt.Sprint(len(t.Members)))}
			for _, c := range []struct {
				what string
				n    int
			}{{"working", working}, {"idle", idle}, {"gone", gone}} {
				if c.n > 0 {
					out = append(out, kv(c.what, fmt.Sprint(c.n)))
				}
			}
			return append(out, kv("gate", orDash(t.Gate)), kv("held", fmt.Sprint(t.Held)), kv("unacked", fmt.Sprint(t.Unacked)), kv("root", home(t.Root)),
				" "+stMuted.Render("enter folds or opens its members"))
		}
		if m.sel == closedRow+t.ID { // a dead team's line: the team's facts
			return []string{" " + lipgloss.NewStyle().Bold(true).Render(t.Name) + stMuted.Render(" · open, all gone"), "",
				kv("active", ago(unit{team: t}.active(), now)+" ago"), kv("gate", orDash(t.Gate)),
				kv("members", fmt.Sprint(len(t.Members))), kv("root", home(t.Root)),
				" " + stMuted.Render("enter shows its members")}
		}
	}
	for i := range m.ps.Closed {
		c := &m.ps.Closed[i]
		if m.sel == closedRow+c.ID { // a closed team's line: the team's facts
			return []string{" " + lipgloss.NewStyle().Bold(true).Render(c.Name) + stMuted.Render(" · closed"), "",
				kv("closed", ago(c.ClosedAt, now)+" ago"), kv("by", cmp.Or(c.ClosedBy, "admin")), kv("gate", orDash(c.Gate)),
				kv("members", fmt.Sprint(len(c.Members))), kv("root", home(c.Root)),
				" " + stMuted.Render("enter shows its members")}
		}
		teams = append(teams, &c.TeamState)
	}
	for _, t := range teams {
		for j := range t.Members {
			if t.Members[j].ID == m.sel {
				team, mem = t, &t.Members[j]
			}
		}
	}
	for i := range m.ps.Solos {
		if m.ps.Solos[i].ID == m.sel {
			solo = &m.ps.Solos[i]
		}
	}
	if mem == nil && solo == nil {
		return []string{" " + stMuted.Render("Nothing selected.")}
	}

	if m.sideTab == sideTail {
		if !m.tailable(m.sel) {
			return []string{" " + stMuted.Render("No tail: piggery has no log of it to read.")}
		}
		var lines []string
		if m.tail.worker == m.sel { // a fetch in flight may still carry the previous worker's
			lines = m.tail.lines
		}
		if len(lines) == 0 {
			return []string{" " + stMuted.Render("No output yet.")}
		}
		if n := min(h, topTailLines); len(lines) > n {
			lines = lines[len(lines)-n:]
		}
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = " " + styleTail(l, w-2)
		}
		return out
	}

	names := m.names()
	var name, role, state, ref string
	var since, created int64
	var unacked int
	if mem != nil {
		name, role, state, since, unacked, ref = mem.Name, mem.Role, mem.State, mem.StateSince, mem.Unacked, mem.ID
		created = mem.CreatedAt
	} else {
		name, role, state, since, unacked, ref = solo.Name, "solo", solo.State, solo.StateSince, solo.Unacked, solo.ID
		created = solo.CreatedAt
	}
	came := "joined " + time.UnixMilli(created).Format("15:04") // a session opened by the Human
	if mem != nil && mem.Headless && mem.SpawnedBy != "" {
		came = "spawned " + time.UnixMilli(created).Format("15:04") + " by " + orDash(names[mem.SpawnedBy])
	}
	icon, col := stateIcon(state)
	out := []string{" " + lipgloss.NewStyle().Bold(true).Render(name) + stMuted.Render(" · "+role+" ") + pill(icon, col, colInk), ""}
	if mem != nil {
		out = append(out, assignmentRows(mem.Assignment, w-10, now)...)
		kind := "session"
		if mem.Headless {
			kind = "headless worker"
		}
		if mem.Harness != "" {
			kind = mem.Harness + " " + kind
		}
		if mem.Gate {
			kind += " · gate"
		}
		out = append(out, kv("team", team.Name), kv("kind", kind), kv("model", modelLabel(mem.Model, mem.Thinking)))
	}
	if ws, ok := m.stats()[ref]; ok {
		ctx := "-"
		if ws.hasCtx {
			ctx = tokens(ws.ctx)
		}
		out = append(out, kv("ctx", ctx), kv("turns", fmt.Sprint(ws.turns)))
	}
	out = append(out, " "+came, kv("since", ago(since, now)), kv("unacked", fmt.Sprint(unacked)))
	if mem != nil {
		if mem.ReportsTo != "" {
			out = append(out, kv("reports", orDash(names[mem.ReportsTo])))
		}
		if mem.LastTurnEnd > 0 {
			out = append(out, kv("last turn", ago(mem.LastTurnEnd, now)+" ago"))
		}
		out = append(out, kv("root", home(team.Root)))
	} else {
		out = append(out, kv("model", modelLabel(solo.Model, "")), kv("cwd", home(solo.Cwd)))
	}
	return append(out, kv("id", stMuted.Render(ref)))
}

// assignmentRows is the member's current task in the sidebar (width n after the 9-cell label):
// its title on at most two lines, who gave it and when, how its reply chain stands (muted), and,
// once the member handed back, the newer mail from the assigner that is not part of the chain
// (a note, or a task sent without op assign). Nothing when it has none.
func assignmentRows(a *core.Assignment, n int, now time.Time) []string {
	if a == nil {
		return nil
	}
	n = max(n, 10)
	indent := strings.Repeat(" ", 10)
	lines := wrapTwo(fmt.Sprintf("#%d %s", a.Seq, a.Title), n)
	out := []string{" " + stMuted.Render(fmt.Sprintf("%-9s", "task")) + lines[0]}
	for _, l := range lines[1:] {
		out = append(out, indent+l)
	}
	out = append(out, indent+"from "+a.From+" · "+ago(a.At, now)+" ago")
	if l := a.Latest; l != nil {
		what := "reply"
		if l.ByMember {
			what = "handed back"
		}
		out = append(out, indent+stMuted.Render(fmt.Sprintf("%s #%d · %s ago", what, l.Seq, ago(l.At, now))))
	}
	if x := a.Newer; x != nil {
		head, tail := fmt.Sprintf("#%d ", x.Seq), " · "+ago(x.At, now)+" ago"
		out = append(out, " "+stMuted.Render(fmt.Sprintf("%-9s", "mail")+head+truncate(x.Title, n-lipgloss.Width(head+tail))+tail))
	}
	return out
}

// wrapTwo breaks s at a space into at most two lines of n cells; the second is cut with … if needed.
func wrapTwo(s string, n int) []string {
	if lipgloss.Width(s) <= n {
		return []string{s}
	}
	w, at, space := 0, 0, 0
	for i, r := range s {
		if w += lipgloss.Width(string(r)); w > n {
			break
		}
		if r == ' ' {
			space = i
		}
		at = i + len(string(r))
	}
	if space > 0 {
		at = space
	}
	return []string{strings.TrimSpace(s[:at]), truncate(strings.TrimSpace(s[at:]), n)}
}

// eventRows are the latest events, newest first (n of them), as cells (time, who, event, target)
// with the event's type.
// eventTime is an event's local time: 15:04:05 today, 01-02 15:04 on an earlier day.
func eventTime(ts int64, now time.Time) string {
	t, now := time.UnixMilli(ts).Local(), now.Local()
	if y, mo, d := t.Date(); y == now.Year() && mo == now.Month() && d == now.Day() {
		return t.Format("15:04:05")
	}
	return t.Format("01-02 15:04")
}

func (m *topModel) eventRows(n int, now time.Time) (rows [][]string, types []string) {
	names := m.names()
	name := func(id string) string {
		if n := names[id]; n != "" || len(id) <= 6 {
			return n
		}
		return id[len(id)-6:]
	}
	for i := len(m.ps.Events) - 1; i >= 0 && len(rows) < n; i-- {
		ev := m.ps.Events[i]
		target := ""
		if ev.RefID != "" && ev.RefID != ev.Participant && ev.RefID != ev.TeamID {
			target = name(ev.RefID)
		}
		rows = append(rows, []string{eventTime(ev.Ts, now), name(ev.Participant), ev.Type, target})
		types = append(types, ev.Type)
	}
	return rows, types
}

// eventStyle is an event row's colour: muted, amber for a refusal, red for an exit.
func eventStyle(typ string) lipgloss.Style {
	switch typ {
	case "denied", "held":
		return lipgloss.NewStyle().Foreground(colWarning)
	case "exited", "gone":
		return lipgloss.NewStyle().Foreground(colError)
	}
	return stMuted
}

// eventsBox is the latest events in a box like the Overview's (3 rows in a short window, 5, or 8 in
// a tall one), or, collapsed, one rule line with the title and the latest event.
func (m *topModel) eventsBox(width, height int, now time.Time) []string {
	n := 8
	switch {
	case height < 30:
		n = 3
	case height < 40:
		n = 5
	}
	if !m.events { // folded: one line of text, no rule, indented like the key lines
		rows, types := m.eventRows(1, now)
		line := " " + stTitle.Render("● Events")
		if len(rows) > 0 {
			latest := truncate(strings.TrimSpace(strings.Join(rows[0], " ")), max(width-lipgloss.Width(" ● Events · "), 0))
			line += stRule.Render(" · ") + eventStyle(types[0]).Render(latest)
		}
		return []string{line}
	}
	rows, types := m.eventRows(n, now)
	inner := width - 2
	w := fitCols(eventCols, rows, inner-3, eventFit)
	lines := make([]string, len(rows))
	if len(rows) == 0 {
		lines = []string{" " + stMuted.Render("No events yet.")}
	}
	for i, r := range rows {
		st := eventStyle(types[i])
		lines[i] = row(r, w, nil, func(int) lipgloss.Style { return st }, false, inner)
	}
	return box(stTitle.Render("Events"), lines, width, max(len(rows), 1)+2)
}

// styleTail colors a tail line (tailLine's forms) cut to n cells: tool calls secondary with
// dim arguments, results muted (errors red), problems amber, assistant text plain.
func styleTail(l string, n int) string {
	switch {
	case strings.HasPrefix(l, "> "):
		name, args, _ := strings.Cut(strings.TrimPrefix(l, "> "), " ")
		head := truncate("▸ "+name, n)
		return lipgloss.NewStyle().Foreground(colTool).Render(head) + stMuted.Render(truncate("  "+args, n-lipgloss.Width(head)))
	case strings.HasPrefix(l, "< "):
		head, text, _ := strings.Cut(strings.TrimPrefix(l, "< "), ": ")
		if name, ok := strings.CutSuffix(head, " error"); ok {
			return lipgloss.NewStyle().Foreground(colError).Render(truncate("✗ "+name+"  "+text, n))
		}
		return stMuted.Render(truncate("✓ "+text, n))
	case strings.HasPrefix(l, "! "):
		return lipgloss.NewStyle().Foreground(colWarning).Render(truncate(l, n))
	case strings.HasPrefix(l, "-- "):
		return stRule.Render(truncate(l, n))
	case strings.HasPrefix(l, "user: "):
		return stMuted.Render(truncate(l, n))
	}
	return truncate(strings.TrimPrefix(l, "assistant: "), n)
}

// ago is the time since ms in one unit: 12s, 4m, 3h, 2d; "-" when unknown.
func ago(ms int64, now time.Time) string {
	if ms == 0 {
		return "-"
	}
	d := max(now.Sub(time.UnixMilli(ms)), 0)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func plural(n int, s string) string {
	if n == 1 {
		return "1 " + s
	}
	return fmt.Sprintf("%d %ss", n, s)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// home shortens a path under the home directory to ~/…
func home(p string) string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		if rel, err := filepath.Rel(h, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return p
}

// truncate cuts plain text to n display cells, with … when cut.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > n-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

// versionNotes are what top's footer can end with, longest first: the daemon's build version and,
// when this binary is another build, the fix (`dev-9057651 (cli dev-99443dc: piggery restart)`, then
// the version alone, amber). None when the daemon does not report a version.
func versionNotes(daemon, self string) (notes []string, mismatch bool) {
	switch {
	case daemon == "":
		return nil, false
	case daemon != self:
		return []string{daemon + " (cli " + self + ": piggery restart)", daemon}, true
	}
	return []string{daemon}, false
}

// withVersion right-aligns the first of notes that fits on the footer's last line, muted (amber
// for a mismatch); when none fits beside the key hints, the version goes and the hints stay whole.
func withVersion(line string, notes []string, mismatch bool, width int) string {
	style := stMuted
	if mismatch {
		style = lipgloss.NewStyle().Foreground(colWarning)
	}
	for _, note := range notes {
		if pad := width - lipgloss.Width(line) - lipgloss.Width(note) - 1; pad >= 2 {
			return line + strings.Repeat(" ", pad) + style.Render(note)
		}
	}
	return line
}

// keyLines is the key footer: two lines by purpose (move and look, act and toggle), or the full
// list under `?`.
func (m *topModel) keyLines(w int) []string {
	if m.help.ShowAll {
		return strings.Split(m.help.View(m.keys), "\n")
	}
	return keyGrid(w, m.keys.look(), m.keys.act())
}

// keyGrid lays the short key lines out as columns: the i-th entry of every line starts at the
// same cell, so the lines read as one block. Keys muted, descriptions subtle, as helpStyles; each
// line is cut to w.
func keyGrid(w int, lines ...[]key.Binding) []string {
	cell := func(b key.Binding) string { return b.Help().Key + " " + b.Help().Desc }
	var cols []int
	for _, l := range lines {
		for i, b := range l {
			if i == len(cols) {
				cols = append(cols, 0)
			}
			cols[i] = max(cols[i], lipgloss.Width(cell(b)))
		}
	}
	out := make([]string, len(lines))
	for li, l := range lines {
		var b strings.Builder
		for i, k := range l {
			if i > 0 {
				b.WriteString("   ")
			}
			b.WriteString(stMuted.Render(k.Help().Key) + " " + stRule.Render(k.Help().Desc))
			if i < len(l)-1 {
				b.WriteString(strings.Repeat(" ", cols[i]-lipgloss.Width(cell(k))))
			}
		}
		out[li] = lipgloss.NewStyle().MaxWidth(w).Render(b.String())
	}
	return out
}
