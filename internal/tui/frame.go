package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// rect is an area of the screen, in cells.
type rect struct{ x, y, w, h int }

func (r rect) has(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

// zone is a clickable area recorded while rendering. Clicks go to the last
// zone recorded at the spot, so overlays win over what is under them.
type zone struct {
	r     rect
	click func() tea.Cmd
}

func (m *model) onClick(r rect, f func() tea.Cmd) {
	m.zones = append(m.zones, zone{r, f})
}

// onKey makes r act like pressing key.
func (m *model) onKey(r rect, key string) {
	m.onClick(r, func() tea.Cmd { return m.keyPress(keyMsg(key)) })
}

// keyMsg is the key press for a key name as KeyPressMsg.String gives it.
func keyMsg(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	if c, ok := strings.CutPrefix(k, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: []rune(c)[0], Mod: tea.ModCtrl}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}

// fit truncates s to width cells with "…" and pads it with spaces to exactly
// width.
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = ansi.Truncate(s, width, "…")
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

// spread puts left and right at the two ends of width cells. If both don't
// fit, right is dropped.
func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return fit(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// center pads s on the left to center it in width cells.
func center(s string, width int) string {
	return strings.Repeat(" ", max(0, (width-lipgloss.Width(s))/2)) + s
}

// boxOpts describes a box's frame: a title at the top left, text at the top
// right, keys at the bottom left, text at the bottom right, and a heavy
// border when it has focus.
type boxOpts struct {
	title, right, keys, bottom string
	focused                    bool
	alert                      bool // a heavy border in the error color, for a question
}

// boxKeys puts a box's keys on its bottom border when they fit a box width
// cells wide; otherwise they come back as lines to end the box's body.
func boxKeys(width int, keys string) (bottom string, lines []string) {
	if lipgloss.Width(keys) <= width-5 {
		return keys, nil
	}
	return "", []string{"", " " + keys}
}

// box draws body in a frame width cells wide and height lines high. Body
// lines are fitted to the inside width; missing lines are blank.
func (m *model) box(width, height int, o boxOpts, body []string) []string {
	h, v, tl, tr, bl, br := "─", "│", "┌", "┐", "└", "┘"
	st := m.th.border
	if o.focused && !m.dimmed || o.alert {
		h, v, tl, tr, bl, br = "━", "┃", "┏", "┓", "┗", "┛"
		st = m.th.focus
	}
	if o.alert {
		st = m.th.err
	}
	inner := width - 2
	edge := func(left, right, l, r string) string {
		if l != "" {
			l = " " + l + " "
		}
		if r != "" {
			r = " " + r + " "
		}
		n := inner - lipgloss.Width(l) - lipgloss.Width(r)
		if n < 0 {
			r, n = "", max(0, inner-lipgloss.Width(l))
			l = ansi.Truncate(l, inner, "…")
			n = inner - lipgloss.Width(l)
		}
		return st.Render(left) + l + st.Render(strings.Repeat(h, n)) + r + st.Render(right)
	}
	lines := make([]string, 0, height)
	lines = append(lines, edge(tl, tr, o.title, o.right))
	for i := 0; i < height-2; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		lines = append(lines, st.Render(v)+fit(line, inner)+st.Render(v))
	}
	return append(lines, edge(bl, br, o.keys, o.bottom))
}

// place writes block (lines) onto screen at x, y, cutting the existing lines
// around it.
func place(screen []string, x, y int, block []string) {
	for i, line := range block {
		row := y + i
		if row < 0 || row >= len(screen) {
			continue
		}
		cur := screen[row]
		if w := lipgloss.Width(cur); w < x {
			cur += strings.Repeat(" ", x-w)
		}
		left := ansi.Truncate(cur, x, "")
		right := ansi.TruncateLeft(cur, x+lipgloss.Width(line), "")
		screen[row] = left + line + right
	}
}

// formatTime formats a task or comment time for display. Tests replace it
// to get stable screens.
var formatTime = func(t time.Time) string {
	t = t.Local()
	if t.Year() != time.Now().Year() {
		return t.Format("Jan 2, 2006")
	}
	return t.Format("Jan 2, 15:04")
}

// truncate cuts s to width cells, ending with "…" if it was cut.
func truncate(s string, width int) string {
	return ansi.Truncate(s, width, "…")
}
