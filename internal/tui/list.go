package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jhdenham/taskship/internal/core"
)

// listKey handles navigation and sorting keys in the list. ok is false for
// keys it doesn't handle.
func (m *model) listKey(k string) (tea.Cmd, bool) {
	switch k {
	case "j", "down":
		m.moveRow(1)
	case "k", "up":
		m.moveRow(-1)
	case "g":
		m.moveRow(-1 << 30)
	case "G":
		m.moveRow(1 << 30)
	case "o":
		m.sortBy = (m.sortBy + 1) % len(sorts)
		m.selectRef(m.selRef)
	default:
		return nil, false
	}
	return nil, true
}

// listColumn is a column of the list: its sort (index into sorts, or -1),
// header, width, and cell.
type listColumn struct {
	sort   int
	header string
	width  int
	cell   func(th theme, t core.Task) string
}

// list draws the task table in r.
func (m *model) list(screen []string, r rect) {
	tasks := m.listTasks()
	inner := r.w - 2
	refW, labelsW := 3, 6
	for _, t := range tasks {
		refW = max(refW, len(t.Ref))
		labelsW = max(labelsW, lipgloss.Width(m.th.labelList(t.Labels)))
	}
	labelsW = min(labelsW, 18)
	cols := []listColumn{
		{2, "REF", refW, func(th theme, t core.Task) string { return th.faint.Bold(true).Render(t.Ref) }},
		{-1, "TITLE", 0, func(th theme, t core.Task) string { return th.plain.Render(t.Title) }},
		{1, "STATUS", 13, func(th theme, t core.Task) string {
			return glyphStyle(th, t.Status).Render(statusGlyph(t.Status)+" ") + th.status(t.Status)
		}},
		{0, "PRIORITY", 10, func(th theme, t core.Task) string { return th.priority(t.Priority) }},
		{-1, "LABELS", labelsW, func(th theme, t core.Task) string { return th.labelList(t.Labels) }},
		{3, "UPDATED", 13, func(th theme, t core.Task) string { return th.faint.Render(formatTime(t.UpdatedAt)) }},
	}
	fixed := 1
	for _, c := range cols {
		fixed += c.width + 2
	}
	if inner-fixed < 24 {
		fixed -= cols[5].width + 2
		cols = cols[:5]
	}
	cols[1].width = inner - fixed + 1

	row := func(th theme, cells []string, marker string) string {
		var b strings.Builder
		b.WriteString(marker)
		for i, c := range cols {
			if i > 0 {
				b.WriteString(th.plain.Render("  "))
			}
			b.WriteString(fitIn(cells[i], c.width, th.plain))
		}
		return b.String()
	}

	focused := !(m.detailOpen && m.focusDetail)
	title := m.th.bold.Render(taskCount(len(tasks)))
	if focused {
		title = m.th.keyBold.Render(taskCount(len(tasks)))
	}
	sortInfo := m.th.faint.Render("sorted by ") + m.th.keyBold.Render(sorts[m.sortBy].name+" ↓") + "  " + m.th.keys("o", "change")

	var header []string
	x := r.x + 2
	for _, c := range cols {
		h := m.th.faint.Bold(true).Render(c.header)
		if c.sort == m.sortBy {
			h = m.th.keyBold.Render(c.header + " ↓")
		}
		header = append(header, h)
		if c.sort >= 0 {
			s := c.sort
			m.onClick(rect{x, r.y + 1, c.width, 1}, func() tea.Cmd {
				m.sortBy = s
				m.selectRef(m.selRef)
				return nil
			})
		}
		x += c.width + 2
	}
	body := []string{row(m.th, header, " ")}

	avail := r.h - 3
	m.listTop = min(m.listTop, m.listRow)
	if m.listRow >= m.listTop+avail {
		m.listTop = m.listRow - avail + 1
	}
	for i := m.listTop; i < len(tasks) && i < m.listTop+avail; i++ {
		t := tasks[i]
		th, marker := m.th, " "
		if i == m.listRow {
			th = m.th.onSelection()
			marker = m.th.keyBold.Render("▌")
		}
		cells := make([]string, len(cols))
		for j, c := range cols {
			cells[j] = c.cell(th, t)
		}
		if i == m.listRow {
			cells[0], cells[1] = th.keyBold.Render(t.Ref), th.bold.Render(t.Title)
		}
		body = append(body, row(th, cells, marker))
		m.onClick(rect{r.x + 1, r.y + 2 + i - m.listTop, inner, 1}, func() tea.Cmd {
			m.listRow = i
			m.openDetail(t.Ref)
			return nil
		})
	}
	place(screen, r.x, r.y, m.box(r.w, r.h, boxOpts{title: title, right: sortInfo, focused: focused}, body))
}

// glyphStyle colors a status glyph like its status.
func glyphStyle(th theme, st core.Status) lipgloss.Style {
	switch st {
	case core.StatusInProgress:
		return th.value
	case core.StatusDone:
		return th.ok
	case core.StatusBacklog, core.StatusCanceled:
		return th.faint
	}
	return th.plain
}
