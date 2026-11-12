package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/editor"
	"github.com/jhdenham/taskship/internal/store"
)

// Editing a task's title and description in a box over the current view.
// ctrl-e moves the edit to $EDITOR as the whole task's Markdown, like
// "taskship task edit".

// startEditTask opens the edit box on t, with the focus on the
// description.
func (m *model) startEditTask(t core.Task) tea.Cmd {
	m.mode, m.editTitle, m.editRef = modeEditTask, false, t.Ref
	m.titleIn.SetValue(t.Title)
	m.titleIn.CursorEnd()
	m.titleIn.Blur()
	// Size the box before filling it, so it doesn't scroll for a smaller
	// default size; drawEditTask sets the exact size.
	m.area.SetWidth(m.modalWidth(110) - 8)
	m.area.SetHeight(m.h)
	m.area.SetValue(strings.Trim(t.Description, "\n"))
	m.area.MoveToBegin()
	return m.area.Focus()
}

// editingTask reports whether the edit box is open, also while asking
// whether to discard it.
func (m *model) editingTask() bool {
	return m.mode == modeEditTask || (m.mode == modeConfirm && m.confirm.back == modeEditTask)
}

func (m *model) editTaskKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "tab", "shift+tab":
		return m.focusEditField(!m.editTitle)
	case "enter":
		if m.editTitle {
			return m.focusEditField(false)
		}
	case "ctrl+s":
		return m.saveTaskEdit()
	case "ctrl+e":
		t, ok := m.task(m.editRef)
		if !ok {
			return nil
		}
		t.Title, t.Description = m.titleIn.Value(), restoreTabs(m.area.Value(), t.Description)
		draft, err := editor.Draft(t.Ref+"-*.md", core.TaskMarkdown(t))
		if err != nil {
			m.fail(err)
			return nil
		}
		m.endEditTask()
		return m.editTask(t, draft)
	case "esc":
		if !m.taskEditChanged() {
			m.endEditTask()
			return nil
		}
		m.confirm = confirm{
			question: m.th.err.Bold(true).Render("Discard your changes to " + m.editRef + "?"),
			yes:      "discard", no: "keep editing", back: modeEditTask,
			onYes: func() tea.Cmd {
				m.endEditTask()
				return nil
			},
			onNo: func() tea.Cmd { return m.focusEditField(m.editTitle) },
		}
		m.mode = modeConfirm
		m.titleIn.Blur() // no cursor while the box asks
		m.area.Blur()
		return nil
	}
	return m.forwardToInput(msg)
}

// focusEditField moves the focus to the title or the description.
func (m *model) focusEditField(title bool) tea.Cmd {
	m.editTitle = title
	if title {
		m.area.Blur()
		return m.titleIn.Focus()
	}
	m.titleIn.Blur()
	return m.area.Focus()
}

func (m *model) endEditTask() {
	m.mode = modeNormal
	m.titleIn.Blur()
	m.area.Blur()
}

// taskEditPatch is the change the form makes to the task.
func (m *model) taskEditPatch() (store.TaskPatch, bool) {
	t, ok := m.task(m.editRef)
	if !ok {
		return store.TaskPatch{}, false
	}
	var patch store.TaskPatch
	if title := strings.TrimSpace(m.titleIn.Value()); title != t.Title {
		patch.Title = &title
	}
	if desc := strings.Trim(restoreTabs(m.area.Value(), t.Description), "\n"); desc != strings.Trim(t.Description, "\n") {
		patch.Description = &desc
	}
	return patch, true
}

func (m *model) taskEditChanged() bool {
	patch, _ := m.taskEditPatch()
	return patch != store.TaskPatch{}
}

// saveTaskEdit saves the form. An invalid change keeps the form open with
// the error shown.
func (m *model) saveTaskEdit() tea.Cmd {
	patch, ok := m.taskEditPatch()
	if !ok {
		m.endEditTask()
		return nil
	}
	if patch == (store.TaskPatch{}) {
		m.endEditTask()
		m.msg = message{[]string{"No changes to " + m.editRef}}
		return nil
	}
	t, err := m.s.UpdateTask(m.ctx, m.editRef, patch)
	if err != nil {
		m.msg = errMsg(m.th, m.editRef+" wasn't saved: "+errText(err))
		return nil
	}
	m.endEditTask()
	m.reload()
	m.msg = okMsg(m.th, "Updated "+t.Ref+": "+t.Title)
	return nil
}

// drawEditTask draws the edit box in the middle of the body area.
func (m *model) drawEditTask(screen []string, area rect) {
	r := rect{w: m.modalWidth(110), h: min(area.h, 30)}
	r.x, r.y = area.x+(area.w-r.w)/2, area.y+(area.h-r.h)/2
	ref := m.editRef
	asking := m.mode == modeConfirm                                // the fields are muted while the box asks
	m.onClick(rect{0, 0, m.w, m.h}, func() tea.Cmd { return nil }) // clicks behind it do nothing
	th := m.th
	inner := r.w - 4
	label := func(name string, focused bool) string {
		if focused {
			return th.keyBold.Render(name)
		}
		return th.faint.Render(name)
	}
	m.titleIn.SetWidth(inner - 5)
	titleBox := m.box(inner, 3, boxOpts{focused: m.editTitle && !asking}, []string{" " + m.titleIn.View()})

	keyText := th.keys("tab", "title/description", "ctrl-s", "save", "ctrl-e", "open in "+editor.Name(), "esc", "cancel")
	if asking {
		keyText = m.confirm.question + "  " + th.questionKeys("y", m.confirm.yes, "n", m.confirm.no)
	}
	bottom, keys := boxKeys(r.w, keyText)
	descH := max(3, r.h-2-len(titleBox)-2-len(keys))
	m.area.SetWidth(inner - 4)
	m.area.SetHeight(descH - 2)
	var desc []string
	for _, l := range strings.Split(m.area.View(), "\n") {
		desc = append(desc, " "+l)
	}
	descBox := m.box(inner, descH, boxOpts{focused: !m.editTitle && !asking}, desc)

	body := []string{" " + label("Title", m.editTitle && !asking)}
	for _, l := range titleBox {
		body = append(body, " "+l)
	}
	body = append(body, " "+label("Description", !m.editTitle && !asking))
	for _, l := range descBox {
		body = append(body, " "+l)
	}
	body = append(body, keys...)

	m.onClick(rect{r.x + 2, r.y + 1, inner, 1 + len(titleBox)}, func() tea.Cmd { return m.focusEditField(true) })
	m.onClick(rect{r.x + 2, r.y + 2 + len(titleBox), inner, 1 + descH}, func() tea.Cmd { return m.focusEditField(false) })
	title := th.keyBold.Render("Edit " + ref)
	if asking {
		title = th.err.Bold(true).Render("Edit " + ref) // the whole frame asks
	}
	place(screen, r.x, r.y, m.box(r.w, r.h, boxOpts{title: title, keys: bottom, focused: true, alert: asking}, body))
}

// restoreTabs puts back the tabs that the text area replaced with four
// spaces (it can't hold tabs): a line of edited that matches a line of
// original with its tabs replaced is that original line. Untouched text,
// such as tab-indented code, comes back as it was.
func restoreTabs(edited, original string) string {
	if !strings.Contains(original, "\t") {
		return edited
	}
	tabbed := map[string]string{}
	for _, l := range strings.Split(original, "\n") {
		if strings.Contains(l, "\t") {
			tabbed[strings.ReplaceAll(l, "\t", "    ")] = l
		}
	}
	lines := strings.Split(edited, "\n")
	for i, l := range lines {
		if o, ok := tabbed[l]; ok {
			lines[i] = o
		}
	}
	return strings.Join(lines, "\n")
}
