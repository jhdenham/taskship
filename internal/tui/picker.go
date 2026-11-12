package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// canSetUpHere reports whether the picker offers to set up the current
// folder: it isn't set up, and init can run there.
func (m *model) canSetUpHere() bool {
	return m.opts.CanSetUp && m.opts.Project == ""
}

func (m *model) openPicker() {
	m.loadProjects()
	m.mode = modeProjects
	m.projCursor = 0
	for i, p := range m.projects {
		if p.name == m.project.Name {
			m.projCursor = i + m.setUpRows()
		}
	}
}

// setUpRows is 1 if the picker has a "Set up this folder" row.
func (m *model) setUpRows() int {
	if m.canSetUpHere() {
		return 1
	}
	return 0
}

func (m *model) projectsKey(k string) tea.Cmd {
	n := len(m.projects) + m.setUpRows()
	switch k {
	case "j", "down":
		m.projCursor = min(n-1, m.projCursor+1)
	case "k", "up":
		m.projCursor = max(0, m.projCursor-1)
	case "g":
		m.projCursor = 0
	case "G":
		m.projCursor = n - 1
	case "enter":
		return m.pickProject(m.projCursor)
	case "i":
		if m.canSetUpHere() {
			return m.pickProject(0)
		}
	case "esc":
		if m.project.ID != "" {
			m.mode = modeNormal
		}
	case "?":
		m.mode = modeHelp
	case "q":
		return tea.Quit
	}
	return nil
}

// pickProject acts on picker row i: set up this folder, or open a project.
func (m *model) pickProject(i int) tea.Cmd {
	if i < m.setUpRows() {
		m.outcome.SetUp = true
		return tea.Quit
	}
	i -= m.setUpRows()
	if i < len(m.projects) {
		m.mode = modeNormal
		m.openProject(m.projects[i].name)
	}
	return nil
}

// picker draws the project picker in the middle of r.
func (m *model) picker(screen []string, r rect) {
	th := m.th
	w := min(64, r.w)
	inner := w - 4
	var lines []string
	rows := 0 // index of the first selectable line
	row := func(i int, text string) string {
		if i == m.projCursor {
			return th.keyBold.Render("▌") + fitIn(th.onSelection().bold.Render(text), inner, th.onSelection().plain)
		}
		return " " + text
	}
	if m.canSetUpHere() {
		lines = append(lines, " "+th.value.Render(m.opts.Folder)+" isn't set up yet.", "")
		rows = len(lines)
		lines = append(lines, row(0, th.keyBold.Render("+ Set up this folder")), "")
	}
	nameW := 8
	for _, p := range m.projects {
		nameW = max(nameW, len(p.name))
	}
	nameW = min(nameW, inner-22)
	line := func(name, prefix, open string) string {
		return fit(name, nameW) + "  " + fit(prefix, 10) + "  " + open
	}
	lines = append(lines, " "+th.faint.Bold(true).Render(line("NAME", "PREFIX", "OPEN TASKS")))

	keys := []string{"j k", "move", "enter", "open"}
	if m.canSetUpHere() {
		keys = append(keys, "i", "set up")
	}
	if m.project.ID != "" {
		keys = append(keys, "esc", "close")
	}
	keys = append(keys, "?", "help", "q", "quit")
	bottom, keyLines := boxKeys(w, th.keys(keys...))

	// Show the projects that fit, scrolled to keep the cursor in view, with
	// a line saying how many more there are.
	room := r.h - 2 - 1 - len(lines) - 1 - len(keyLines) // borders, blank lines, keys
	window := len(m.projects)
	if window > room {
		window = max(1, room-1) // a line for "↑ 2 more  ↓ 5 more"
	}
	if c := m.projCursor - m.setUpRows(); c >= 0 {
		m.projTop = min(m.projTop, c)
		if c >= m.projTop+window {
			m.projTop = c - window + 1
		}
	}
	m.projTop = max(0, min(m.projTop, len(m.projects)-window))
	first := len(lines)
	for i := m.projTop; i < m.projTop+window; i++ {
		p := m.projects[i]
		text := th.bold.Render(fit(p.name, nameW)) + "  " + th.faint.Render(fit(p.prefix, 10)) + "  " + fmt.Sprint(p.open)
		lines = append(lines, row(i+m.setUpRows(), text))
	}
	if above, below := m.projTop, len(m.projects)-m.projTop-window; above+below > 0 {
		var more []string
		if above > 0 {
			more = append(more, fmt.Sprintf("↑ %d more", above))
		}
		if below > 0 {
			more = append(more, fmt.Sprintf("↓ %d more", below))
		}
		lines = append(lines, " "+th.faint.Render(strings.Join(more, "  ")))
	}

	h := len(lines) + 4 + len(keyLines) // borders and the blank lines around the rows
	x, y := r.x+(r.w-w)/2, r.y+max(0, (r.h-h)/3)
	if m.project.ID != "" {
		m.onKey(rect{0, 0, m.w, m.h}, "esc")
	}
	click := func(line, i int) {
		m.onClick(rect{x + 1, y + 2 + line, w - 2, 1}, func() tea.Cmd {
			if m.projCursor == i {
				return m.pickProject(i)
			}
			m.projCursor = i
			return nil
		})
	}
	if m.canSetUpHere() {
		click(rows, 0)
	}
	for i := m.projTop; i < m.projTop+window; i++ {
		click(first+i-m.projTop, i+m.setUpRows())
	}
	body := append([]string{""}, lines...)
	body = append(body, "")
	body = append(body, keyLines...)
	place(screen, x, y, m.box(w, h, boxOpts{title: th.keyBold.Render("Projects"), keys: bottom, focused: true}, body))
}
