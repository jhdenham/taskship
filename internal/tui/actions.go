package tui

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/dispatch"
	"github.com/jhdenham/taskship/internal/display"
	"github.com/jhdenham/taskship/internal/editor"
	"github.com/jhdenham/taskship/internal/store"
	"github.com/jhdenham/taskship/internal/workspace"
)

// Status, priority and labels.

func (m *model) statusMenu(t core.Task) {
	var names []string
	cur := 0
	for i, st := range core.Statuses {
		names = append(names, statusName(st))
		if st == t.Status {
			cur = i
		}
	}
	m.openMenu("status", "Status of "+t.Ref, names, cur, func(i int) tea.Cmd {
		m.setStatus(t, core.Statuses[i])
		return nil
	})
}

func (m *model) priorityMenu(t core.Task) {
	var names []string
	cur := 0
	for i, p := range core.Priorities {
		names = append(names, string(p))
		if p == t.Priority {
			cur = i
		}
	}
	m.openMenu("priority", "Priority of "+t.Ref, names, cur, func(i int) tea.Cmd {
		p := core.Priorities[i]
		if p == t.Priority {
			return nil
		}
		if _, err := m.s.UpdateTask(m.ctx, t.Ref, store.TaskPatch{Priority: &p}); err != nil {
			m.fail(err)
			return nil
		}
		m.reload()
		m.msg = okMsg(m.th, fmt.Sprintf("Set %s's priority to %s", t.Ref, p))
		return nil
	})
}

func (m *model) openMenu(field, title string, options []string, current int, pick func(int) tea.Cmd) {
	m.menu = menu{field: field, title: title, options: options, verb: "set", cursor: current, current: current, pick: pick}
	m.mode = modeMenu
}

func (m *model) menuKey(k string) tea.Cmd {
	switch k {
	case "esc", "q":
		m.mode = modeNormal
	case "j", "down":
		m.menu.cursor = min(len(m.menu.options)-1, m.menu.cursor+1)
	case "k", "up":
		m.menu.cursor = max(0, m.menu.cursor-1)
	case "enter":
		m.mode = modeNormal
		return m.menu.pick(m.menu.cursor)
	default:
		for i, o := range m.menu.options {
			if strings.HasPrefix(o, k) {
				m.menu.cursor = i
				break
			}
		}
	}
	return nil
}

// popupAt places a w×h box in the middle of r, the area between the tabs
// and the footer, like the edit and comment boxes. Clicks outside the box
// cancel it.
func (m *model) popupAt(r rect, w, h int) (x, y int) {
	x, y = r.x+(r.w-w)/2, r.y+(r.h-h)/2
	x = max(0, min(x, m.w-w))
	y = max(1, min(y, m.h-2-h))
	m.onKey(rect{0, 0, m.w, m.h}, "esc")
	m.onClick(rect{x, y, w, h}, func() tea.Cmd { return nil })
	return x, y
}

// changingField is the field of the selected task that a menu or the
// labels box is changing, or "".
func (m *model) changingField() string {
	switch {
	case m.mode == modeMenu:
		return m.menu.field
	case m.mode == modeInput && m.inputKind == inputLabels:
		return "labels"
	}
	return ""
}

// drawLabels draws the labels box.
func (m *model) drawLabels(screen []string, r rect) {
	title := "Labels of " + m.inputRef
	w := min(r.w, 44)
	m.input.SetWidth(w - 5)
	bottom, keys := boxKeys(w, m.th.keys("enter", "save", "esc", "cancel"))
	lines := append([]string{" " + m.input.View(), "", " " + m.th.faint.Render("Commas or spaces separate; empty clears.")}, keys...)
	h := len(lines) + 2
	x, y := m.popupAt(r, w, h)
	place(screen, x, y, m.box(w, h, boxOpts{title: m.th.keyBold.Render(title), keys: bottom, focused: true}, lines))
}

// drawMenu draws the open menu.
func (m *model) drawMenu(screen []string, r rect) {
	w := max(28, len(m.menu.title)+6)
	for i, o := range m.menu.options {
		if i < len(m.menu.hints) {
			w = max(w, len(o)+len(m.menu.hints[i])+6)
		}
	}
	var lines []string
	for i, o := range m.menu.options {
		th, marker := m.th, " "
		if i == m.menu.cursor {
			th = m.th.onSelection()
			marker = m.th.keyBold.Render("▌")
		}
		cur := ""
		switch {
		case i == m.menu.current:
			cur = th.faint.Render("current")
		case i < len(m.menu.hints):
			cur = th.faint.Render(m.menu.hints[i])
		}
		lines = append(lines, marker+spreadIn(th.plain.Render(" "+o), cur, w-3, th.plain))
	}
	bottom, keys := boxKeys(w, m.th.keys("enter", m.menu.verb, "esc", "cancel"))
	lines = append(lines, keys...)
	h := len(lines) + 2
	x, y := m.popupAt(r, w, h)
	for i := range m.menu.options {
		m.onClick(rect{x + 1, y + 1 + i, w - 2, 1}, func() tea.Cmd {
			m.mode = modeNormal
			return m.menu.pick(i)
		})
	}
	place(screen, x, y, m.box(w, h, boxOpts{title: m.th.keyBold.Render(m.menu.title), keys: bottom, focused: true}, lines))
}

// setStatus moves a task to another status.
func (m *model) setStatus(t core.Task, st core.Status) {
	if st == t.Status {
		return
	}
	if _, err := m.s.UpdateTask(m.ctx, t.Ref, store.TaskPatch{Status: &st}); err != nil {
		m.fail(err)
		return
	}
	m.selRef = t.Ref // the selection follows the task to its new column
	m.reload()
	var more []string
	if st == core.StatusCanceled && !m.showCanceled {
		more = append(more, m.th.keys("C", "shows canceled tasks"))
	}
	m.msg = okMsg(m.th, fmt.Sprintf("Moved %s to %s", t.Ref, statusName(st))+strings.Repeat(" ", 3*len(more))+strings.Join(more, ""))
}

// Inputs: the filter, labels and quick add.

func (m *model) startInput(kind inputKind, value, ref string) {
	m.mode, m.inputKind, m.inputRef = modeInput, kind, ref
	m.prevFilter, m.filterErr = m.filter, ""
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.SetWidth(max(20, m.w-50))
	m.input.Focus()
}

func (m *model) inputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.inputKind == inputFilter {
			m.filter = m.prevFilter
			m.selectRef(m.selRef)
		}
		m.endInput()
		return nil
	case "enter":
		return m.submitInput(false)
	case "ctrl+e":
		if m.inputKind == inputNew {
			return m.submitInput(true)
		}
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.inputChanged()
	return cmd
}

func (m *model) endInput() {
	m.mode = modeNormal
	m.input.Blur()
}

// inputChanged applies the filter as it is typed.
func (m *model) inputChanged() {
	if m.mode != modeInput || m.inputKind != inputFilter {
		return
	}
	f, err := parseFilter(m.input.Value())
	if err != nil {
		m.filterErr = errText(err)
		return
	}
	m.filter, m.filterErr = f, ""
	m.selectRef(m.selRef)
}

func (m *model) submitInput(edit bool) tea.Cmd {
	value := strings.TrimSpace(m.input.Value())
	switch m.inputKind {
	case inputFilter:
		if m.filterErr != "" {
			return nil
		}
		m.endInput()
	case inputLabels:
		m.endInput()
		var labels []string
		for _, l := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
			if l = strings.TrimPrefix(l, "#"); l != "" {
				labels = append(labels, l)
			}
		}
		labels, err := core.NormalizeLabels(labels)
		if err != nil {
			m.fail(err)
			return nil
		}
		if _, err := m.s.UpdateTask(m.ctx, m.inputRef, store.TaskPatch{Labels: &labels}); err != nil {
			m.fail(err)
			return nil
		}
		m.reload()
		if len(labels) == 0 {
			m.msg = okMsg(m.th, "Cleared "+m.inputRef+"'s labels")
		} else {
			m.msg = okMsg(m.th, m.inputRef+" labels: "+m.th.labelList(labels))
		}
	case inputNew:
		m.endInput()
		if value == "" {
			return nil
		}
		t, err := m.s.CreateTask(m.ctx, m.project.Name, core.Task{Title: value, Status: m.quickAddCol, Priority: core.PriorityNone})
		if err != nil {
			m.fail(err)
			return nil
		}
		m.reload()
		m.selectRef(t.Ref)
		m.msg = okMsg(m.th, "Added "+t.Ref+": "+t.Title)
		if edit {
			return m.editTask(t, "")
		}
	}
	return nil
}

// inputLines are the status lines while an input is open.
func (m *model) inputLines() []string {
	width := m.w - 2
	switch m.inputKind {
	case inputFilter:
		hint := m.th.faint.Render("matches title and ref")
		if m.filterErr != "" {
			hint = m.th.err.Render(m.filterErr)
		}
		return []string{spread(m.th.keyBold.Render("/ ")+m.input.View(), hint, width)}
	case inputLabels:
		return nil // the labels box
	}
	if m.view == viewList {
		return []string{m.th.keyBold.Render("+ ") + m.th.faint.Render("new in "+statusName(m.quickAddCol)+": ") + m.input.View()}
	}
	return nil
}

func (m *model) startQuickAdd() tea.Cmd {
	m.quickAddCol = core.StatusTodo
	if m.view == viewBoard {
		m.quickAddCol = m.columns()[m.col]
	}
	if m.showArchived {
		m.showArchived = false
		m.selectRef(m.selRef)
	}
	if !m.wide() {
		m.detailOpen = false
	}
	m.focusDetail = false
	m.startInput(inputNew, "", "")
	return nil
}

// Confirmations.

func (m *model) askArchive(t core.Task) {
	archive := t.ArchivedAt == nil
	verb, past := "Archive", "Archived"
	if !archive {
		verb, past = "Restore", "Restored"
	}
	m.confirm = confirm{
		question: m.th.value.Bold(true).Render(verb+" "+t.Ref) + fmt.Sprintf(" %q?", t.Title),
		yes:      strings.ToLower(verb), no: "keep",
		onYes: func() tea.Cmd {
			if _, err := m.s.SetTaskArchived(m.ctx, t.Ref, archive); err != nil {
				m.fail(err)
				return nil
			}
			m.reload()
			if archive {
				m.msg = okMsg(m.th, past+" "+m.th.value.Render(t.Ref)+"   "+m.th.keys("A", "shows archived tasks"))
			} else {
				m.msg = okMsg(m.th, past+" "+m.th.value.Render(t.Ref))
			}
			return nil
		},
	}
	m.mode = modeConfirm
}

func (m *model) askDeleteComment(c core.Comment) {
	ref := m.detailRef
	m.confirm = confirm{
		question: m.th.err.Bold(true).Render(fmt.Sprintf("Delete comment #%d?", c.Number)),
		yes:      "delete", no: "keep", comment: c.Number,
		onYes: func() tea.Cmd {
			if _, err := m.s.DeleteComment(m.ctx, ref, c.Number); err != nil {
				m.fail(err)
				return nil
			}
			m.pick = -1
			m.reload() // reloads the comments too
			m.msg = okMsg(m.th, fmt.Sprintf("Deleted comment #%d", c.Number))
			return nil
		},
	}
	m.mode = modeConfirm
}

func (m *model) confirmKey(k string) tea.Cmd {
	switch k {
	case "y":
		m.mode = modeNormal
		return m.confirm.onYes()
	case "n", "esc":
		m.mode = m.confirm.back
		if m.confirm.onNo != nil {
			return m.confirm.onNo()
		}
	}
	return nil
}

// Editing a task in $EDITOR.

type editedMsg struct {
	ref, draft string
	err        error
}

// editTask opens a task in the editor, from draft if it isn't empty (to
// reopen invalid changes, or an edit moved from the box, which has set
// editBase). The TUI suspends until the editor exits.
func (m *model) editTask(t core.Task, draft string) tea.Cmd {
	if draft == "" {
		m.editBase = t
		var err error
		if draft, err = editor.Draft(t.Ref+"-*.md", core.TaskMarkdown(t)); err != nil {
			m.fail(err)
			return nil
		}
	}
	m.editDraft, m.editDraftFor = draft, t.Ref
	return tea.ExecProcess(editor.Command(draft), func(err error) tea.Msg { return editedMsg{t.Ref, draft, err} })
}

func (m *model) edited(msg editedMsg) tea.Cmd {
	if msg.err != nil {
		// The draft may hold the only copy of the text (from the edit box,
		// or invalid changes being retried), so it stays for e or esc.
		m.mode = modeEditError
		m.msg = errMsg(m.th, fmt.Sprintf("%s failed: %v", editor.Name(), msg.err),
			"Your text is kept in "+display.Path(msg.draft)+".")
		return nil
	}
	src, err := os.ReadFile(msg.draft)
	if err != nil {
		m.cleanUpDraft()
		m.fail(err)
		return nil
	}
	cur, err := m.s.GetTask(m.ctx, msg.ref)
	if err != nil {
		m.cleanUpDraft()
		m.fail(err)
		return nil
	}
	// The draft is compared with the task as it was when the edit started,
	// so only the fields changed in it are saved.
	base := m.editBase
	if base.Ref != cur.Ref {
		base = cur
	}
	patch, err := store.PatchFromMarkdown(base, string(src))
	if err == nil && patch == (store.TaskPatch{}) {
		m.cleanUpDraft()
		m.msg = message{[]string{"No changes to " + cur.Ref}}
		return nil
	}
	if err == nil && conflicts(base, cur, patch) {
		m.askOverwrite(cur.Ref, modeEditError, "keep my draft",
			func() tea.Cmd { return m.saveEdited(msg.ref, patch) },
			func() tea.Cmd {
				m.msg = errMsg(m.th, msg.ref+" changed elsewhere while you edited it. Your text is kept in "+display.Path(msg.draft)+".")
				return nil
			})
		return nil
	}
	if err != nil {
		m.mode = modeEditError
		m.msg = errMsg(m.th, msg.ref+" wasn't saved: "+errText(err))
		return nil
	}
	return m.saveEdited(msg.ref, patch)
}

// saveEdited saves a change made in the editor. An invalid change keeps
// the draft for e or esc.
func (m *model) saveEdited(ref string, patch store.TaskPatch) tea.Cmd {
	t, err := m.s.UpdateTask(m.ctx, ref, patch)
	if err != nil {
		m.mode = modeEditError
		m.msg = errMsg(m.th, ref+" wasn't saved: "+errText(err)) // t is empty after a failed update
		return nil
	}
	m.cleanUpDraft()
	m.reload()
	m.msg = okMsg(m.th, "Updated "+t.Ref+": "+t.Title)
	return nil
}

func (m *model) editErrorKey(k string) tea.Cmd {
	switch k {
	case "e":
		m.mode, m.msg = modeNormal, message{}
		t, ok := m.task(m.editDraftFor)
		if !ok {
			m.cleanUpDraft()
			return nil
		}
		return m.editTask(t, m.editDraft)
	case "esc":
		m.cleanUpDraft()
		m.mode, m.msg = modeNormal, message{}
	}
	return nil
}

func (m *model) cleanUpDraft() {
	if m.editDraft != "" {
		os.Remove(m.editDraft)
		m.editDraft, m.editDraftFor = "", ""
	}
}

// Comments.

type commentEditedMsg struct {
	draft string
	err   error
}

// startComment opens the comment box on t: a new comment for number 0,
// else an edit of comment number with its body.
func (m *model) startComment(t core.Task, number int, body string) tea.Cmd {
	cs, err := m.s.ListComments(m.ctx, t.ID)
	if err != nil {
		m.fail(err)
		return nil
	}
	// The box shows the comments before the new or edited one.
	m.earlier = cs
	if number > 0 {
		m.earlier = slices.DeleteFunc(slices.Clone(cs), func(c core.Comment) bool { return c.Number >= number })
	}
	// Numbers are never reused, so after a deleted last comment this is a
	// guess; the store assigns the real one.
	m.commentNumber = 1
	for _, c := range cs {
		m.commentNumber = max(m.commentNumber, c.Number+1)
	}
	m.mode, m.editComment, m.editRef, m.showEarlier = modeComment, number, t.Ref, false
	m.area.SetWidth(m.modalWidth(90) - 8)
	m.area.SetHeight(m.h)
	m.area.SetValue(body)
	m.commentOrig = body
	return m.area.Focus()
}

// commenting reports whether the comment box is open, also while asking
// whether to discard it.
func (m *model) commenting() bool {
	return m.mode == modeComment || (m.mode == modeConfirm && m.confirm.back == modeComment)
}

// drawComment draws the comment box in the middle of r: the text area and
// the keys, and above them, when there are earlier comments, a rule that
// shows or hides them (ctrl-r). Shown, they are the most recent that fit,
// read-only.
func (m *model) drawComment(screen []string, r rect) {
	th := m.th
	title := fmt.Sprintf("New comment #%d on %s", m.commentNumber, m.editRef)
	if m.editComment > 0 {
		title = fmt.Sprintf("Edit comment #%d on %s", m.editComment, m.editRef)
	}
	w := m.modalWidth(90)
	textW := w - 4
	bottom, keys := boxKeys(w, th.keys("enter", "new line", "ctrl-s", "save", "ctrl-e", "open in "+editor.Name(), "esc", "cancel"))
	alert := m.mode == modeConfirm
	if alert {
		bottom, keys = boxKeys(w, m.confirm.question+"  "+th.questionKeys("y", m.confirm.yes, "n", m.confirm.no))
	}

	areaH := 8 // lines to write in
	maxH := min(r.h, 30)
	var history []string
	if len(m.earlier) > 0 {
		label := th.keys("ctrl-r", fmt.Sprintf("show %d earlier %s", len(m.earlier), plural(len(m.earlier), "comment", "comments")))
		if m.showEarlier {
			areaH = 6
			room := maxH - 2 - (areaH + 2) - len(keys) - 1 // the 1 is the rule
			history = m.recentComments(textW, room)
			label = th.keys("ctrl-r", "hide comments")
		}
		if alert {
			label = th.faint.Render(ansi.Strip(label)) // ctrl-r doesn't work while the box asks
		}
		rule := th.faint.Render("── ") + label + " "
		history = append(history, rule+th.faint.Render(strings.Repeat("─", max(3, textW-lipgloss.Width(rule)))))
	}
	areaH = max(3, min(areaH, maxH-2-len(history)-2-len(keys)))

	m.area.SetWidth(textW - 4)
	m.area.SetHeight(areaH)
	var area []string
	for _, l := range strings.Split(m.area.View(), "\n") {
		area = append(area, " "+l)
	}
	var body []string
	for _, l := range history {
		body = append(body, " "+l)
	}
	for _, l := range m.box(textW, areaH+2, boxOpts{focused: !alert}, area) { // muted while asking
		body = append(body, " "+l)
	}
	for _, l := range keys {
		body = append(body, " "+l)
	}

	h := len(body) + 2
	x, y := r.x+(r.w-w)/2, r.y+(r.h-h)/2
	m.onClick(rect{0, 0, m.w, m.h}, func() tea.Cmd { return nil }) // clicks behind it do nothing
	if len(history) > 0 {
		m.onKey(rect{x + 2, y + len(history), textW, 1}, "ctrl+r") // the rule
	}
	titleStyle := th.keyBold
	if alert {
		titleStyle = th.err.Bold(true) // the whole frame asks
	}
	place(screen, x, y, m.box(w, h, boxOpts{title: titleStyle.Render(title), keys: bottom, focused: true, alert: alert}, body))
}

// plural picks one or many by n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// recentComments renders the latest of m.earlier in at most room lines,
// oldest first, starting with "↑ N earlier comments" when some don't fit.
func (m *model) recentComments(width, room int) []string {
	th := m.th
	var blocks [][]string
	for _, c := range m.earlier {
		block := []string{commentHeading(th, c)}
		for _, l := range renderMarkdown(c.Body, width, th) {
			block = append(block, th.faint.Render(ansi.Strip(l))) // muted: context, not for editing
		}
		blocks = append(blocks, block)
	}
	var shown []string
	n := 0 // comments shown, from the newest
	for i := len(blocks) - 1; i >= 0; i-- {
		need := len(blocks[i])
		if n > 0 {
			need++ // blank line between comments
		}
		if i > 0 {
			need++ // room for the "earlier" line if older ones don't fit
		}
		if len(shown)+need > room {
			if n == 0 {
				// Even the newest doesn't fit: its first lines, then "…".
				keep := max(1, room-1-min(i, 1))
				shown = append(blocks[i][:min(keep, len(blocks[i]))], th.faint.Render("…"))
				n = 1
			}
			break
		}
		if n > 0 {
			shown = append([]string{""}, shown...)
		}
		shown = append(append([]string{}, blocks[i]...), shown...)
		n++
	}
	if older := len(blocks) - n; older > 0 {
		shown = append([]string{th.faint.Render(fmt.Sprintf("↑ %d earlier %s", older, plural(older, "comment", "comments")))}, shown...)
	}
	return shown
}

// modalWidth is the width of a text box: up to most columns, within the
// screen.
func (m *model) modalWidth(most int) int {
	return min(most, m.w-2)
}

func (m *model) commentKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.commentText() == m.commentOrig {
			m.endComment()
			return nil
		}
		q := "Discard this comment?"
		if m.editComment > 0 {
			q = fmt.Sprintf("Discard your changes to comment #%d?", m.editComment)
		}
		m.confirm = confirm{
			question: m.th.err.Bold(true).Render(q),
			yes:      "discard", no: "keep writing", back: modeComment,
			onYes: func() tea.Cmd {
				m.endComment()
				return nil
			},
			onNo: func() tea.Cmd { return m.area.Focus() },
		}
		m.mode = modeConfirm
		m.area.Blur() // no cursor while the box asks
		return nil
	case "ctrl+s":
		return m.saveComment(m.commentText())
	case "ctrl+r":
		m.showEarlier = !m.showEarlier
		return nil
	case "ctrl+e":
		draft, err := editor.Draft(m.editRef+"-comment-*.md", m.commentText())
		if err != nil {
			m.fail(err)
			return nil
		}
		return tea.ExecProcess(editor.Command(draft), func(err error) tea.Msg { return commentEditedMsg{draft, err} })
	}
	var cmd tea.Cmd
	m.area, cmd = m.area.Update(msg)
	return cmd
}

// commentText is the comment box's text, with the tabs of the comment
// being edited put back.
func (m *model) commentText() string {
	return restoreTabs(m.area.Value(), m.commentOrig)
}

func (m *model) endComment() {
	m.mode = modeNormal
	m.area.Blur()
}

func (m *model) commentEdited(msg commentEditedMsg) tea.Cmd {
	defer os.Remove(msg.draft)
	if msg.err != nil {
		m.fail(fmt.Errorf("the editor failed: %w", msg.err))
		return nil
	}
	src, err := os.ReadFile(msg.draft)
	if err != nil {
		m.fail(err)
		return nil
	}
	m.area.SetValue(string(src))
	return m.saveComment(string(src))
}

func (m *model) saveComment(text string) tea.Cmd {
	body, err := core.CleanCommentBody(text)
	if err != nil {
		m.msg = errMsg(m.th, "The comment is empty. Write something, or press esc to cancel.")
		return nil
	}
	if m.editComment == 0 {
		c, err := m.s.AddComment(m.ctx, m.editRef, body, "")
		if err != nil {
			m.fail(err)
			return nil
		}
		m.msg = okMsg(m.th, fmt.Sprintf("Added comment #%d to %s", c.Number, m.editRef))
	} else {
		if _, err := m.s.UpdateComment(m.ctx, m.editRef, m.editComment, body); err != nil {
			m.fail(err)
			return nil
		}
		m.msg = okMsg(m.th, fmt.Sprintf("Updated comment #%d", m.editComment))
	}
	m.endComment()
	m.reload() // reloads the detail view's comments too
	return nil
}

// Launch.

// launch checks that an agent can run on t, showing what's wrong in place,
// then asks whether to run or refine it. A backlog task starts on refine.
func (m *model) launch(t core.Task) tea.Cmd {
	if _, ok := m.prepare(t, core.RunModeRun); !ok {
		return nil
	}
	modes := []core.RunMode{core.RunModeRun, core.RunModeRefine}
	cursor := 0
	if t.Status == core.StatusBacklog {
		cursor = 1
	}
	m.menu = menu{title: "Launch an agent on " + t.Ref, options: []string{"run", "refine"},
		hints: []string{"work on it", "make it ready"}, verb: "launch", cursor: cursor, current: -1,
		pick: func(i int) tea.Cmd {
			plan, ok := m.prepare(t, modes[i])
			if !ok {
				return nil
			}
			m.outcome.Launch = &plan
			return tea.Quit
		}}
	m.mode = modeMenu
	return nil
}

// prepare plans an agent run on t for mode, or shows what's wrong.
func (m *model) prepare(t core.Task, mode core.RunMode) (dispatch.Plan, bool) {
	plan, err := dispatch.Prepare(m.ctx, m.s, t, mode, ".", false)
	if err == nil {
		err = plan.Ready()
	}
	var nf *workspace.NoFolderError
	switch {
	case errors.As(err, &nf) && nf.Missing != "":
		m.msg = errMsg(m.th, m.th.value.Bold(true).Render(nf.Project)+m.th.err.Render("'s folder "+display.Path(nf.Missing)+" no longer exists."),
			"If it moved, run "+m.th.keyBold.Render("taskship init")+" in the new location, then press "+m.th.keyBold.Render("r")+" again.")
	case errors.As(err, &nf):
		m.msg = errMsg(m.th, m.th.value.Bold(true).Render(nf.Project)+m.th.err.Render(" has no folder on this machine."),
			"Run "+m.th.keyBold.Render("taskship init")+" in its repository, then press "+m.th.keyBold.Render("r")+" again.")
	case errors.Is(err, dispatch.ErrAgentMissing):
		m.msg = errMsg(m.th, "Claude Code isn't installed, or isn't on your PATH.",
			"Install it from https://claude.com/claude-code, then press "+m.th.keyBold.Render("r")+" again.")
	case err != nil:
		m.fail(err)
	default:
		return plan, true
	}
	return dispatch.Plan{}, false
}

// Mouse wheel.

func (m *model) wheel(msg tea.MouseWheelMsg) tea.Cmd {
	if m.mode != modeNormal || m.project.ID == "" {
		return nil
	}
	delta := 1
	if msg.Button == tea.MouseWheelUp {
		delta = -1
	}
	if m.detailOpen && (!m.wide() || m.detailRect.has(msg.X, msg.Y)) {
		m.detailScroll = max(0, m.detailScroll+delta*3)
		return nil
	}
	m.moveRow(delta)
	return nil
}
