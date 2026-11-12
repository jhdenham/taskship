package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/display"
)

// openDetail opens the detail view on a task and gives it the focus.
func (m *model) openDetail(ref string) {
	m.selRef = ref
	m.detailOpen, m.focusDetail = true, true
	if m.detailRef != ref {
		m.showDetail(ref)
	} else {
		m.loadComments()
	}
}

// showDetail shows a task in the detail view from the top.
func (m *model) showDetail(ref string) {
	m.detailRef, m.detailScroll, m.pick = ref, 0, -1
	m.loadComments()
}

func (m *model) loadComments() {
	t, ok := m.task(m.detailRef)
	if !ok {
		m.comments = nil
		return
	}
	cs, err := m.s.ListComments(m.ctx, t.ID)
	if err != nil {
		m.fail(err)
		return
	}
	m.comments = cs
	m.pick = min(m.pick, len(cs)-1)
}

// detailKey handles keys while the detail view has the focus.
func (m *model) detailKey(k string) tea.Cmd {
	switch k {
	case "esc":
		if m.pick >= 0 {
			m.pick = -1
			return nil
		}
		m.detailOpen, m.focusDetail = false, false
		m.selectRef(m.detailRef)
	case "tab":
		if m.wide() {
			m.focusDetail = false
		}
	case "j", "down":
		m.detailScroll++
	case "k", "up":
		m.detailScroll = max(0, m.detailScroll-1)
	case "g":
		m.detailScroll = 0
	case "G":
		m.detailScroll = 1 << 30
	case "[":
		if len(m.comments) > 0 {
			if m.pick < 0 {
				m.pick = len(m.comments)
			}
			m.pick = max(0, m.pick-1)
			m.scrollToPick = true
		}
	case "]":
		if len(m.comments) > 0 {
			m.pick = min(len(m.comments)-1, m.pick+1)
			m.scrollToPick = true
		}
	case "e":
		if m.pick >= 0 {
			c := m.comments[m.pick]
			t, _ := m.task(m.detailRef)
			return m.startComment(t, c.Number, c.Body)
		}
		return m.taskKey(k)
	case "d":
		if m.pick >= 0 {
			m.askDeleteComment(m.comments[m.pick])
		}
	default:
		return m.taskKey(k)
	}
	return nil
}

// detail draws the detail pane for the task in m.detailRef in r.
func (m *model) detail(screen []string, r rect) {
	t, ok := m.task(m.detailRef)
	if !ok {
		return
	}
	m.detailRect = r
	th := m.th
	focused := m.focusDetail || !m.wide()
	textW := r.w - 4

	// The actions, pinned at the bottom.
	type action struct{ key, label string }
	actions := []action{{"r", "launch agent"}, {"e", "edit"}, {"s", "status"}, {"p", "priority"}, {"#", "labels"},
		{"c", "comment"}, {"a", m.archiveWord()}, {"y", "copy ref"}}
	var actionLines []string
	type placed struct {
		line, x, w int
		key        string
	}
	var spots []placed
	cur, curW := "", 0
	for i, a := range actions {
		s := th.keyBold.Render(a.key) + " " + th.faint.Render(a.label)
		if i == 0 {
			s = th.keyBold.Render(a.key + " " + a.label)
		}
		sep := th.faint.Render(" · ")
		w := lipgloss.Width(s)
		if curW > 0 && curW+3+w > textW {
			actionLines = append(actionLines, cur)
			cur, curW = "", 0
		}
		if curW > 0 {
			cur += sep
			curW += 3
		}
		spots = append(spots, placed{len(actionLines), curW, w, a.key})
		cur += s
		curW += w
	}
	actionLines = append(actionLines, cur)

	content, commentAt := m.detailContent(t, textW)
	areaH := r.h - 2 - 1 - len(actionLines)
	maxScroll := max(0, len(content)-areaH)
	if m.scrollToPick && m.pick >= 0 {
		start, end := commentAt[m.pick], len(content)
		if m.pick+1 < len(commentAt) {
			end = commentAt[m.pick+1]
		}
		if start < m.detailScroll || end > m.detailScroll+areaH {
			m.detailScroll = start - 1
		}
		m.scrollToPick = false
	}
	m.detailScroll = max(0, min(m.detailScroll, maxScroll))

	var body []string
	marked := -1 // the field a menu or the labels box is changing
	if l, ok := m.fieldLines[m.changingField()]; ok {
		marked = l
	}
	for i := m.detailScroll; i < len(content) && i < m.detailScroll+areaH; i++ {
		lead := " "
		if i == marked {
			lead = th.keyBold.Render("▌")
		}
		body = append(body, lead+content[i])
	}
	for len(body) < areaH {
		body = append(body, "")
	}
	body = append(body, " "+th.faint.Render(strings.Repeat("─", textW)))
	for _, l := range actionLines {
		body = append(body, " "+l)
	}
	m.onClick(rect{r.x, r.y, r.w, r.h}, func() tea.Cmd {
		m.focusDetail = true
		return nil
	})
	// The field values open their menus or input, like their keys.
	for name, key := range map[string]string{"status": "s", "priority": "p", "labels": "#"} {
		if i, ok := m.fieldLines[name]; ok && i >= m.detailScroll && i < m.detailScroll+areaH {
			a := rect{r.x + 2 + m.fieldX, r.y + 1 + i - m.detailScroll, textW - m.fieldX, 1}
			m.anchors["field:"+name] = a
			m.onKey(a, key)
		}
	}
	actionsY := r.y + 1 + areaH + 1
	for _, s := range spots {
		m.onKey(rect{r.x + 2 + s.x, actionsY + s.line, s.w, 1}, s.key)
	}

	scroll := ""
	switch {
	case maxScroll == 0:
	case m.detailScroll == 0:
		scroll = th.faint.Render("↓ ") + th.keys("j k", "scroll")
	case m.detailScroll == maxScroll:
		scroll = th.faint.Render("↑ ") + th.keys("j k", "scroll")
	default:
		scroll = th.faint.Render("↕ ") + th.keys("j k", "scroll")
	}
	back := th.keys("esc", "close")
	m.onKey(rect{r.x + r.w - 2 - lipgloss.Width(back), r.y, lipgloss.Width(back), 1}, "esc")

	title := th.keyBold.Render(t.Ref) + th.faint.Render(" · details")
	place(screen, r.x, r.y, m.box(r.w, r.h, boxOpts{title: title, right: back, bottom: scroll, focused: focused}, body))
}

// detailContent returns the scrolling part of the detail view and the line
// where each comment starts.
func (m *model) detailContent(t core.Task, width int) (lines []string, commentAt []int) {
	th := m.th
	for _, l := range wrapWords(t.Title, width) {
		lines = append(lines, th.bold.Render(l))
	}
	lines = append(lines, "")
	// Values line up two spaces after the longest name shown.
	nameW := len("priority")
	if t.CompletedAt != nil {
		nameW = len("completed")
	}
	m.fieldLines, m.fieldX = map[string]int{}, nameW+2
	field := func(name, v string) {
		m.fieldLines[name] = len(lines)
		label := th.faint.Render(fmt.Sprintf("%-*s", nameW+2, name))
		if m.changingField() == name {
			label = th.keyBold.Render(fmt.Sprintf("%-*s", nameW+2, name))
		}
		lines = append(lines, label+v)
	}
	field("status", th.status(t.Status))
	field("priority", th.priority(t.Priority))
	switch {
	case len(t.Labels) > 0:
		field("labels", th.labelList(t.Labels))
	default:
		field("labels", th.faint.Render("—"))
	}
	field("updated", formatTime(t.UpdatedAt))
	if t.CompletedAt != nil {
		by := ""
		if t.CompletedBy != nil {
			by = th.faint.Render(" by ") + t.CompletedBy.String()
		}
		field("completed", formatTime(*t.CompletedAt)+by)
	}
	if t.ArchivedAt != nil {
		field("archived", formatTime(*t.ArchivedAt))
	}

	lines = append(lines, "", m.rule("Description", width))
	if strings.TrimSpace(t.Description) == "" {
		lines = append(lines, th.faint.Render("No description. ")+th.keyBold.Render("e")+th.faint.Render(" opens the editor."))
	} else {
		lines = append(lines, renderMarkdown(t.Description, width, th)...)
	}

	lines = append(lines, "")
	if len(m.comments) == 0 {
		lines = append(lines, m.rule("Comments", width))
		{
			lines = append(lines, th.faint.Render("No comments yet. ")+th.keyBold.Render("c")+th.faint.Render(" adds one."))
		}
	} else {
		lines = append(lines, m.rule(fmt.Sprintf("Comments (%d)", len(m.comments)), width))
	}
	for i, c := range m.comments {
		if i > 0 {
			lines = append(lines, "")
		}
		commentAt = append(commentAt, len(lines))
		bar := ""
		if i == m.pick {
			bar = th.keyBold.Render("▌")
		}
		if m.mode == modeConfirm && m.confirm.comment == c.Number {
			lines = append(lines, bar+m.confirm.question+" "+th.keys("y", m.confirm.yes, "n", m.confirm.no))
		} else {
			lines = append(lines, bar+commentHeading(th, c))
		}
		for _, l := range renderMarkdown(c.Body, width-lipgloss.Width(bar), th) {
			lines = append(lines, bar+l)
		}
	}
	return lines, commentAt
}

// rule is a muted section heading across width: "── Description ─────".
func (m *model) rule(title string, width int) string {
	return m.th.faint.Render(display.Rule(title, width))
}

// commentHeading is a comment's heading line: "#2 · Nov 11, 09:20 · edited".
func commentHeading(th theme, c core.Comment) string {
	h := th.value.Bold(true).Render(fmt.Sprintf("#%d", c.Number)) + th.faint.Render(" · "+formatTime(c.CreatedAt))
	if c.Edited() {
		h += th.faint.Render(" · edited")
	}
	if c.Run != nil {
		h += th.faint.Render(" · " + c.Run.String())
	}
	return h
}
