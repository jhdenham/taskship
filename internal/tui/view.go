package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jhdenham/taskship/internal/editor"
)

func (m *model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// render draws the whole screen and records the click zones.
func (m *model) render() string {
	m.zones = m.zones[:0]
	m.anchors = map[string]rect{}
	if m.w == 0 {
		return ""
	}
	if m.w < minWidth || m.h < minHeight {
		return m.tooSmall()
	}
	screen := make([]string, m.h)
	screen[0] = m.header()

	status := m.statusLines()
	footer := m.footer()
	bottom := append(status, footer)
	if len(status) == 0 {
		bottom = []string{"", footer}
	}
	body := rect{x: 1, y: 3, w: m.w - 2, h: m.h - 3 - len(bottom)}
	for i, l := range bottom {
		screen[m.h-len(bottom)+i] = " " + l
	}
	m.recordFooterZones(m.h-1, footer)

	// While a box is open, what is behind it shows no focus.
	m.dimmed = m.modalOpen()
	switch {
	case m.project.ID == "" && !m.hasProjects:
		m.noProjects(screen, body)
	case m.project.ID == "":
		// The picker is drawn below as an overlay.
	default:
		screen[2] = m.tabs()
		m.body(screen, body)
	}
	m.dimmed = false
	if m.editingTask() {
		m.drawEditTask(screen, body)
	}
	if m.commenting() {
		m.drawComment(screen, body)
	}
	if m.mode == modeProjects || (m.project.ID == "" && m.hasProjects) {
		m.picker(screen, body)
	}
	if m.mode == modeMenu {
		m.drawMenu(screen, body)
	}
	if m.mode == modeInput && m.inputKind == inputLabels {
		m.drawLabels(screen, body)
	}
	if m.mode == modeHelp {
		// The help covers the tabs and the (empty) footer too, for room at
		// 80×24.
		m.help(screen, rect{body.x, 1, body.w, m.h - 1})
	}
	return strings.Join(screen, "\n")
}

// header is the top line: the project, the filter state, and "? help".
func (m *model) header() string {
	left := m.th.keyBold.Render("◆ taskship") + "  "
	switch {
	case m.project.ID == "":
		left += m.th.faint.Render(m.opts.Folder)
	case m.w < 100:
		left = m.th.key.Render("◆") + " " + m.th.faint.Render(m.project.Name)
	default:
		left += m.th.faint.Render(m.project.Name)
	}
	var right []string
	if m.project.ID != "" {
		if m.filter.active() {
			matching, total := m.counts()
			f := m.th.faint.Render("filter ") + m.th.value.Bold(true).Render(m.filter.text) + m.th.faint.Render(fmt.Sprintf(" · %d of %d", matching, total))
			switch {
			case m.w >= 100 && !m.modalOpen():
				f += m.th.faint.Render(" tasks · ") + m.th.keyBold.Render("esc") + m.th.faint.Render(" clears")
			case m.w >= 100:
				f += m.th.faint.Render(" tasks")
			}
			right = append(right, f)
		}
		if m.showArchived {
			right = append(right, m.th.faint.Render("showing ")+m.th.value.Bold(true).Render("archived"))
		}
		if m.showCanceled {
			right = append(right, m.th.faint.Render("showing ")+m.th.value.Bold(true).Render("canceled"))
		}
		switch {
		case m.stale:
			right = append(right, m.th.keyBold.Render("↻ Changes detected"))
		case m.updated:
			right = append(right, m.th.ok.Render("↻ Updated   ")) // as wide as "Up to date"
		default:
			right = append(right, m.th.faint.Render("✓ Up to date"))
		}
	}
	// "? help" goes while a box is open, like the footer: ? doesn't open
	// the help then (the project picker lists it with its own keys).
	if !m.modalOpen() {
		help := m.th.keyBold.Render("?") + " " + m.th.faint.Render("help")
		right = append(right, help)
		m.onKey(rect{m.w - 1 - lipgloss.Width(help), 0, lipgloss.Width(help), 1}, "?")
	}
	return " " + spread(left, strings.Join(right, "  "), m.w-2)
}

// tabs is the view switcher line.
func (m *model) tabs() string {
	tab := func(key, name string, on bool) string {
		if on {
			return m.th.sel.Bold(true).Foreground(m.th.key.GetForeground()).Render(" " + key + " " + name + " ")
		}
		return m.th.faint.Render(" " + key + " " + name + " ")
	}
	board := tab("1", "Board", m.view == viewBoard)
	list := tab("2", "List", m.view == viewList)
	m.onKey(rect{1, 2, lipgloss.Width(board), 1}, "1")
	m.onKey(rect{1 + lipgloss.Width(board) + 1, 2, lipgloss.Width(list), 1}, "2")
	return " " + board + " " + list
}

// body draws the board or list, the detail pane, or an empty state.
func (m *model) body(screen []string, r rect) {
	matching, total := m.counts()
	switch {
	case total == 0 && m.showArchived:
		m.centered(screen, r, "No archived tasks in "+m.th.value.Bold(true).Render(m.project.Name)+".",
			m.th.keyBold.Render("A")+m.th.faint.Render(" goes back to open tasks"))
		return
	case total == 0 && m.mode != modeInput:
		m.centered(screen, r, "No tasks in "+m.th.value.Bold(true).Render(m.project.Name)+" yet.",
			m.th.faint.Render("Press ")+m.th.keyBold.Render("n")+m.th.faint.Render(" to add one."))
		return
	case matching == 0 && total > 0 && m.mode != modeInput:
		m.centered(screen, r, "No tasks match "+m.th.value.Bold(true).Render(m.filter.text)+".",
			m.th.keyBold.Render("Esc")+m.th.faint.Render(" clears the filter."))
		return
	}
	main := r
	if m.detailOpen {
		if !m.wide() {
			m.detail(screen, r)
			return
		}
		dw := m.detailWidth()
		main.w = r.w - dw - 1
		m.detail(screen, rect{r.x + main.w + 1, r.y, dw, r.h})
	}
	if m.view == viewBoard {
		m.board(screen, main)
	} else {
		m.list(screen, main)
	}
}

// modalOpen reports whether a box is open over the views.
func (m *model) modalOpen() bool {
	switch m.mode {
	case modeHelp, modeProjects, modeMenu, modeEditTask, modeComment:
		return true
	case modeInput:
		return m.inputKind == inputLabels
	}
	return m.editingTask() || m.commenting()
}

// detailWidth is the width of the detail pane.
func (m *model) detailWidth() int {
	if !m.wide() {
		return m.w - 2
	}
	return max(60, (m.w-2)*5/12)
}

// centered writes lines in the middle of r, a blank line apart.
func (m *model) centered(screen []string, r rect, lines ...string) {
	y := r.y + r.h/2 - len(lines)
	for i, l := range lines {
		screen[y+i*2] = strings.Repeat(" ", r.x) + center(l, r.w)
	}
}

func (m *model) tooSmall() string {
	lines := []string{
		m.th.bold.Render("Terminal too small."),
		"",
		m.th.faint.Render("Taskship needs at least ") + m.th.value.Render(fmt.Sprintf("%d×%d", minWidth, minHeight)) +
			m.th.faint.Render("; this one is ") + m.th.value.Render(fmt.Sprintf("%d×%d", m.w, m.h)) + m.th.faint.Render("."),
		m.th.faint.Render("Make the window bigger, or press ") + m.th.keyBold.Render("q") + m.th.faint.Render(" to quit."),
	}
	screen := make([]string, m.h)
	top := max(0, (m.h-len(lines))/2)
	for i, l := range lines {
		if top+i < m.h {
			screen[top+i] = center(l, m.w)
		}
	}
	return strings.Join(screen, "\n")
}

func (m *model) noProjects(screen []string, r rect) {
	if m.opts.CanSetUp {
		m.centered(screen, r, m.th.bold.Render("No projects yet."),
			m.th.faint.Render("A project holds the tasks of one repository or folder."),
			m.th.faint.Render("Press ")+m.th.keyBold.Render("i")+m.th.faint.Render(" to set up ")+m.th.value.Render(m.opts.Folder)+m.th.faint.Render("."))
		return
	}
	m.centered(screen, r, m.th.bold.Render("No projects yet."),
		m.th.faint.Render("A project holds the tasks of one repository or folder."),
		m.th.faint.Render("Run ")+m.th.keyBold.Render("taskship init")+m.th.faint.Render(" in a repository to set one up."))
}

// statusLines are the lines above the footer: an input, a question, or a
// message.
func (m *model) statusLines() []string {
	switch m.mode {
	case modeInput:
		return m.inputLines()
	case modeConfirm:
		if m.modalOpen() {
			return nil // the question is on the box's border
		}
		if m.confirm.comment == 0 {
			return []string{m.confirm.question + " " + m.th.keys("y", m.confirm.yes, "n", m.confirm.no)}
		}
	}
	// Long messages wrap, indented under their first word.
	var lines []string
	for _, l := range m.msg.lines {
		if lipgloss.Width(l) <= m.w-2 {
			lines = append(lines, l)
			continue
		}
		plain := ansi.Strip(l)
		style := m.th.plain
		if strings.HasPrefix(plain, "✗") {
			style = m.th.err
		}
		for i, w := range wrapWords(plain, m.w-4) {
			if i > 0 {
				w = "  " + w
			}
			lines = append(lines, style.Render(w))
		}
	}
	return lines
}

// footer is the key line for the current view and mode.
func (m *model) footer() string {
	narrow := m.w < 100
	if m.modalOpen() {
		// A box lists its own keys, and the others don't work while it's
		// open.
		return ""
	}
	switch m.mode {
	case modeInput:
		switch m.inputKind {
		case inputFilter:
			return m.th.keys("enter", "apply", "esc", "cancel", "#web", "label", "status:todo", "", "prio:high", "")
		case inputNew:
			return m.th.keys("enter", "add", "ctrl-e", "add and open in "+editor.Name(), "esc", "cancel")
		}
	case modeConfirm:
		return m.th.keys("y", m.confirm.yes, "n", m.confirm.no)
	case modeEditError:
		return m.th.keys("e", "reopen "+editor.Name()+" with your changes", "esc", "discard them")
	}
	if m.project.ID == "" {
		if !m.hasProjects && m.opts.CanSetUp {
			return m.th.keys("i", "set up", "?", "help", "q", "quit")
		}
		return m.th.keys("?", "help", "q", "quit")
	}
	matching, total := m.counts()
	switch {
	case total == 0 && m.showArchived:
		return m.th.keys("A", "open tasks", "1 2", "board/list", "?", "help", "q", "quit")
	case total == 0:
		return m.th.keys("n", "new task", "P", "projects", "?", "help", "q", "quit")
	case matching == 0:
		return m.th.keys("esc", "clear filter", "/", "edit filter", "?", "help", "q", "quit")
	}
	if m.detailFocused() {
		keys := []string{"esc", "close", "j k", "scroll", "[ ]", "pick comment"}
		if m.pick >= 0 {
			keys = append(keys, "e", "edit comment", "d", "delete comment")
		}
		if m.wide() {
			keys = append(keys, "tab", "board", "/", "filter")
		}
		return m.th.keys(append(keys, "?", "help", "q", "quit")...)
	}
	var keys []string
	if m.filter.active() {
		keys = append(keys, "esc", "clear filter", "/", "edit filter")
	}
	switch {
	case m.view == viewList && narrow:
		keys = append(keys, "j k", "move", "enter", "open", "o", "sort", "n", "new", "/", "filter", "A", "archived")
	case m.view == viewList:
		keys = append(keys, "j k", "move", "enter", "open", "o", "sort", "n", "new task", "s", "status", "p", "priority",
			"#", "labels", "a", m.archiveWord(), "/", "filter", "A", "show archived", "1 2", "board/list", "P", "projects")
	case narrow:
		keys = append(keys, "hjkl", "move", "enter", "open", "n", "new", "H L", "move card", "/", "filter")
	default:
		keys = append(keys, "h j k l", "move", "enter", "open", "n", "new task", "H L", "move card", "s", "status",
			"p", "priority", "#", "labels", "a", m.archiveWord(), "/", "filter", "C", "show canceled", "1 2", "board/list", "P", "projects")
	}
	return m.th.keys(append(keys, "?", "help", "q", "quit")...)
}

// archiveWord is what a does in the current view.
func (m *model) archiveWord() string {
	if m.showArchived {
		return "restore"
	}
	return "archive"
}

// recordFooterZones makes the single keys in the footer clickable.
func (m *model) recordFooterZones(y int, footer string) {
	plain := ansi.Strip(footer)
	x := 1
	for _, part := range strings.Split(plain, "  ") {
		k, _, _ := strings.Cut(part, " ")
		switch k {
		case "enter", "esc", "tab", "?", "q", "n", "o", "/", "A", "C", "P", "s", "p", "#", "a", "i", "y", "e", "d", "R":
			m.onKey(rect{x, y, lipgloss.Width(part), 1}, k)
		case "ctrl-s", "ctrl-e", "ctrl-r":
			m.onKey(rect{x, y, lipgloss.Width(part), 1}, strings.Replace(k, "-", "+", 1))
		}
		x += lipgloss.Width(part) + 2
	}
}
