package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jhdenham/taskship/internal/core"
)

// boardKey handles navigation keys on the board. ok is false for keys it
// doesn't handle.
func (m *model) boardKey(k string) (tea.Cmd, bool) {
	cols := m.columns()
	switch k {
	case "h", "left":
		m.col = max(0, m.col-1)
		m.selChanged()
	case "l", "right":
		m.col = min(len(cols)-1, m.col+1)
		m.selChanged()
	case "j", "down":
		m.moveRow(1)
	case "k", "up":
		m.moveRow(-1)
	case "g":
		m.moveRow(-1 << 30)
	case "G":
		m.moveRow(1 << 30)
	case "H", "L":
		t, ok := m.selected()
		if !ok {
			return nil, true
		}
		i := m.col - 1
		if k == "L" {
			i = m.col + 1
		}
		if i < 0 || i >= len(cols) {
			return nil, true
		}
		m.setStatus(t, cols[i])
	default:
		return nil, false
	}
	return nil, true
}

// board draws the board's columns in r.
func (m *model) board(screen []string, r rect) {
	cols := m.columns()
	n := len(cols)
	colW, extra := (r.w-(n-1))/n, (r.w-(n-1))%n
	x := r.x
	for i, st := range cols {
		w := colW
		if i < extra {
			w++
		}
		m.drawColumn(screen, rect{x, r.y, w, r.h}, i, st)
		x += w + 1
	}
}

func (m *model) drawColumn(screen []string, r rect, i int, st core.Status) {
	tasks, total := m.column(st)
	focused := i == m.col && !(m.detailOpen && m.focusDetail)
	name := m.th.bold.Render(statusName(st))
	if focused {
		name = m.th.keyBold.Render(statusName(st))
	}
	count := fmt.Sprint(total)
	if m.filter.active() {
		count = fmt.Sprintf("%d of %d", len(tasks), total)
		if r.w < 26 {
			count = fmt.Sprintf("%d/%d", len(tasks), total)
		}
	}
	m.onClick(r, func() tea.Cmd {
		m.col, m.focusDetail = i, false
		m.selChanged()
		return nil
	})

	inner := r.w - 2
	avail := r.h - 2
	var body []string
	y := r.y + 1
	if m.mode == modeInput && m.inputKind == inputNew && m.quickAddCol == st {
		m.input.SetWidth(inner - 5)
		card := m.box(inner, 3, boxOpts{title: m.th.faint.Render("new in " + statusName(st)), focused: true}, []string{" " + m.input.View()})
		body = append(body, card...)
		avail -= 3
		y += 3
	}
	if len(tasks) == 0 {
		place(screen, r.x, r.y, m.box(r.w, r.h, boxOpts{title: name + " " + m.th.faint.Render(count), focused: focused},
			append(body, m.th.faint.Render(" "+map[bool]string{true: "no matches", false: "—"}[m.filter.active()]))))
		return
	}

	// Scroll so the selected card shows.
	heights := make([]int, len(tasks))
	for j, t := range tasks {
		heights[j] = 4
		if len(t.Labels) > 0 {
			heights[j] = 5
		}
	}
	sel := m.rows[st]
	top := min(m.colTop[st], sel)
	fits := func(from, to int) bool { // cards from..to, plus a "more" line if any follow
		h := 0
		for j := from; j <= to; j++ {
			h += heights[j]
		}
		if to < len(tasks)-1 {
			h++
		}
		return h <= avail
	}
	for top < sel && !fits(top, sel) {
		top++
	}
	m.colTop[st] = top

	used := 0
	for j := top; j < len(tasks); j++ {
		need := heights[j]
		if j < len(tasks)-1 {
			need++ // room for "↓ N more" if the next card doesn't fit
		}
		if used+need > avail && j > top {
			body = append(body, m.th.faint.Render(fmt.Sprintf(" ↓ %d more", len(tasks)-j)))
			break
		}
		t := tasks[j]
		selected := i == m.col && j == sel
		card := m.card(t, inner, selected)
		m.onClick(rect{r.x + 1, y + used, inner, len(card)}, func() tea.Cmd {
			m.col, m.rows[st] = i, j
			m.openDetail(tasks[j].Ref)
			return nil
		})
		body = append(body, card...)
		used += len(card)
	}
	place(screen, r.x, r.y, m.box(r.w, r.h, boxOpts{title: name + " " + m.th.faint.Render(count), focused: focused}, body))
}

// card draws a task's card width cells wide.
func (m *model) card(t core.Task, width int, selected bool) []string {
	th := m.th
	tw := width - 4
	ref, title := th.faint.Bold(true), th.plain
	if selected {
		th = th.onSelection()
		ref, title = th.keyBold, th.bold
	}
	prio := ""
	if t.Priority != core.PriorityNone {
		prio = th.priority(t.Priority)
	}
	line := func(s string) string {
		return th.plain.Render(" ") + fitIn(s, tw, th.plain) + th.plain.Render(" ")
	}
	top := line(spreadIn(ref.Render(t.Ref), prio, tw, th.plain))
	if lipgloss.Width(t.Ref)+1+lipgloss.Width(prio) > tw {
		// Use the right-hand space rather than drop the priority.
		top = th.plain.Render(" ") + spreadIn(ref.Render(t.Ref), prio, tw+1, th.plain)
	}
	lines := []string{top, line(title.Render(truncate(t.Title, tw)))}
	if len(t.Labels) > 0 {
		lines = append(lines, line(th.labelList(t.Labels)))
	}
	return m.box(width, len(lines)+2, boxOpts{focused: selected}, lines)
}

// fitIn is fit with padding drawn in pad's style.
func fitIn(s string, width int, pad lipgloss.Style) string {
	s = truncate(s, width)
	return s + pad.Render(strings.Repeat(" ", max(0, width-lipgloss.Width(s))))
}

// spreadIn is spread with the gap drawn in pad's style.
func spreadIn(left, right string, width int, pad lipgloss.Style) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return left
	}
	return left + pad.Render(strings.Repeat(" ", gap)) + right
}
