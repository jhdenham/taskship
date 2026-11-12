package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jhdenham/taskship/internal/editor"
)

type helpSection struct {
	title string
	keys  [][2]string
}

var (
	helpMove  = helpSection{"Move", [][2]string{{"h j k l", "move (or arrows)"}, {"g G", "first / last"}, {"tab", "board ↔ detail"}}}
	helpViews = helpSection{"Views", [][2]string{{"1 2", "board / list"}, {"P", "projects"}, {"/", "filter"},
		{"o", "sort (list)"}, {"C", "show canceled"}, {"A", "show archived"}, {"R", "refresh"}}}
	helpTasks = helpSection{"Tasks", [][2]string{{"enter", "open"}, {"n", "new task"}, {"H L", "move left / right"},
		{"s", "status"}, {"p", "priority"}, {"#", "labels"}, {"e", "edit (ctrl-e: EDITOR)"}, {"a", "archive / restore"}}}
	helpDetail = helpSection{"Detail", [][2]string{{"r", "launch agent"}, {"c", "add comment"}, {"[ ]", "pick a comment"},
		{"e d", "edit / delete it"}, {"y", "copy ref"}, {"esc", "close"}}}
	helpGeneral = helpSection{"General", [][2]string{{"?", "this help"}, {"q", "quit"}}}
	helpMouse   = helpSection{"Mouse", [][2]string{{"click", "select, open, press"}, {"wheel", "scroll"}}}
	helpBoxes   = helpSection{"Boxes", [][2]string{{"tab", "title ↔ description"}, {"ctrl-s", "save"},
		{"ctrl-e", "open in EDITOR"}, {"ctrl-r", "earlier comments"}}}
	// At 80 columns, tab (board ↔ detail) doesn't apply: the detail
	// replaces the board.
	helpMoveNarrow = helpSection{"Move", helpMove.keys[:2]}
)

// help draws the key overlay over r.
func (m *model) help(screen []string, r rect) {
	th := m.th
	cols := [][]helpSection{{helpMove, helpViews}, {helpTasks, helpBoxes}, {helpDetail, helpMouse, helpGeneral}}
	note := []string{th.faint.Render("Mouse is captured: ") + th.keyBold.Render("Shift-drag") +
		th.faint.Render(" selects text (") + th.keyBold.Render("Option-drag") + th.faint.Render(" in iTerm2).")}
	if r.w < 110 {
		// Two columns; the mouse goes in the note at the bottom.
		cols = [][]helpSection{{helpMoveNarrow, helpViews, helpBoxes}, {helpTasks, helpDetail}}
		note = append([]string{th.keyBold.Render("Click") + th.faint.Render(" selects, opens and presses; the ") +
			th.keyBold.Render("wheel") + th.faint.Render(" scrolls.")}, note...)
	}
	colW := 34
	var blocks [][]string
	height := 0
	for _, sections := range cols {
		var lines []string
		for i, s := range sections {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, th.bold.Underline(true).Render(s.title))
			for _, k := range s.keys {
				lines = append(lines, th.keyBold.Render(fit(k[0], 12))+strings.Replace(k[1], "EDITOR", editor.Name(), 1))
			}
		}
		blocks = append(blocks, lines)
		height = max(height, len(lines))
	}
	var body []string
	for i := 0; i < height; i++ {
		var b strings.Builder
		b.WriteString(" ")
		for _, lines := range blocks {
			line := ""
			if i < len(lines) {
				line = lines[i]
			}
			b.WriteString(line + strings.Repeat(" ", max(0, colW-lipgloss.Width(line))))
		}
		body = append(body, b.String())
	}
	body = append(body, "")
	for _, l := range note {
		body = append(body, " "+l)
	}

	w := min(r.w, colW*len(cols)+4)
	h := min(r.h, len(body)+2)
	x, y := r.x+(r.w-w)/2, r.y+max(0, (r.h-h)/4)
	m.onKey(rect{0, 0, m.w, m.h}, "esc")
	place(screen, x, y, m.box(w, h, boxOpts{title: th.keyBold.Render("Keys"), keys: th.keys("esc", "close"), focused: true}, body))
}
