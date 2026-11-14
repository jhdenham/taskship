package tui

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/editor"
)

// The Notes view: a project's notes as a grid of equal-height cards, newest
// first, a pane for the whole note, a box to write one, and making a task
// from one.

// Cards are at least noteCardMinW wide and show notePreview lines.
const noteCardMinW, notePreview = 36, 6

// timeAgo formats when a note was written for its card: "5 min ago",
// "yesterday", "Nov 10". Tests replace it to get stable screens.
var timeAgo = func(t time.Time) string {
	t = t.Local()
	now := time.Now()
	d := now.Sub(t)
	y, mo, day := now.Date()
	today := time.Date(y, mo, day, 0, 0, 0, 0, now.Location())
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case !t.Before(today):
		h := int(d.Hours())
		return fmt.Sprintf("%d %s ago", h, plural(h, "hour", "hours"))
	case !t.Before(today.AddDate(0, 0, -1)):
		return "yesterday"
	case t.Year() == now.Year():
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

// shownNotes are the notes shown, newest first: in the Notes view, those
// that match its filter; in the board's lane, all of them.
func (m *model) shownNotes() []core.Note {
	if m.view != viewNotes {
		return m.notes
	}
	words := strings.Fields(strings.ToLower(m.noteFilter))
	var notes []core.Note
	for _, n := range m.notes {
		body := strings.ToLower(n.Body)
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(body, w) }) {
			notes = append(notes, n)
		}
	}
	return notes
}

// note returns the loaded note with number n.
func (m *model) note(n int) (core.Note, bool) {
	i := slices.IndexFunc(m.notes, func(x core.Note) bool { return x.Number == n })
	if i < 0 {
		return core.Note{}, false
	}
	return m.notes[i], true
}

// selectedNote returns the selected note in the grid.
func (m *model) selectedNote() (core.Note, bool) {
	notes := m.shownNotes()
	if m.noteRow < len(notes) {
		return notes[m.noteRow], true
	}
	return core.Note{}, false
}

// targetNote is the note actions apply to: the one in the pane when it is
// open, else the selected one.
func (m *model) targetNote() (core.Note, bool) {
	if m.noteOpen {
		return m.note(m.noteNum)
	}
	return m.selectedNote()
}

// selectNote selects the note with number n if it shows, else keeps the
// position within bounds.
func (m *model) selectNote(n int) {
	notes := m.shownNotes()
	if i := slices.IndexFunc(notes, func(x core.Note) bool { return x.Number == n }); i >= 0 {
		m.noteRow = i
	}
	m.noteRow = max(0, min(m.noteRow, len(notes)-1))
	m.noteSelChanged()
}

// noteSelChanged records the selected note after the selection moved. The
// pane beside the grid follows it.
func (m *model) noteSelChanged() {
	n, ok := m.selectedNote()
	if !ok {
		return
	}
	m.selNote = n.Number
	if m.noteOpen && m.wide() && !m.focusNote && m.noteNum != n.Number {
		m.noteNum, m.noteScroll = n.Number, 0
	}
}

// noteFocused reports whether keys go to the note pane: it has the focus
// beside the grid, or it replaces the grid.
func (m *model) noteFocused() bool {
	return m.noteOpen && (m.focusNote || !m.wide())
}

func (m *model) openNote(n core.Note) {
	m.selNote, m.detailOpen = n.Number, false
	if m.noteNum != n.Number || !m.noteOpen {
		m.noteNum, m.noteScroll = n.Number, 0
	}
	m.noteOpen, m.focusNote = true, true
}

// noteCols is how many cards fit across the grid.
func (m *model) noteCols() int {
	w := m.w - 2
	if m.noteOpen && m.wide() {
		w -= m.detailWidth() + 1
	}
	return max(1, (w+1)/(noteCardMinW+1))
}

// notePaneKey handles keys while the note pane has the focus.
func (m *model) notePaneKey(k string) tea.Cmd {
	switch k {
	case "esc":
		m.noteOpen, m.focusNote = false, false
		m.selectNote(m.noteNum)
	case "tab":
		if m.wide() {
			m.focusNote = false
		}
	case "j", "down":
		m.noteScroll++
	case "k", "up":
		m.noteScroll = max(0, m.noteScroll-1)
	case "g":
		m.noteScroll = 0
	case "G":
		m.noteScroll = 1 << 30
	default:
		return m.noteKey(k)
	}
	return nil
}

// notesKey handles keys in the Notes view.
func (m *model) notesKey(k string) tea.Cmd {
	if m.noteFocused() {
		return m.notePaneKey(k)
	}
	cols := m.noteCols()
	switch k {
	case "esc":
		switch {
		case m.noteOpen:
			m.noteOpen = false
		case m.noteFilter != "":
			m.noteFilter = ""
			m.selectNote(m.selNote)
		}
	case "h", "left":
		if m.noteRow%cols > 0 {
			m.moveNote(-1)
		}
	case "l", "right":
		if m.noteRow%cols < cols-1 {
			m.moveNote(1)
		}
	case "j", "down":
		if m.noteRow+cols < len(m.shownNotes()) {
			m.moveNote(cols)
		}
	case "k", "up":
		if m.noteRow >= cols {
			m.moveNote(-cols)
		}
	case "g":
		m.moveNote(-1 << 30)
	case "G":
		m.moveNote(1 << 30)
	case "tab":
		if m.noteOpen {
			m.focusNote = true
		}
	case "enter":
		if n, ok := m.selectedNote(); ok {
			m.openNote(n)
		}
	default:
		return m.noteKey(k)
	}
	return nil
}

// noteKey handles the keys that act on the target note.
func (m *model) noteKey(k string) tea.Cmd {
	n, ok := m.targetNote()
	if !ok {
		return nil
	}
	switch k {
	case "t":
		return m.startTaskFromNote(n)
	case "e":
		return m.startNoteBox(n)
	case "d":
		m.askDeleteNote(n)
	case "y":
		m.msg = okMsg(m.th, "Copied note")
		return tea.SetClipboard(n.Body)
	}
	return nil
}

// moveNote moves the selection by delta notes, within bounds.
func (m *model) moveNote(delta int) {
	m.noteRow = max(0, min(m.noteRow+delta, len(m.shownNotes())-1))
	m.noteSelChanged()
}

func (m *model) askDeleteNote(n core.Note) {
	m.confirm = confirm{
		question: m.th.err.Bold(true).Render("Delete this note?"),
		yes:      "delete", no: "keep", note: n.Number,
		onYes: func() tea.Cmd {
			if _, err := m.s.DeleteNote(m.ctx, m.project.Name, n.Number); err != nil {
				m.fail(err)
				return nil
			}
			m.reload() // the pane closes, or moves to the selection beside the grid
			m.msg = okMsg(m.th, "Deleted the note")
			return nil
		},
	}
	m.mode = modeConfirm
}

// The grid.

// notesBody draws the Notes view: the grid and the note pane, or an empty
// state.
func (m *model) notesBody(screen []string, r rect) {
	notes := m.shownNotes()
	switch {
	case len(m.notes) == 0 && m.mode != modeNoteBox:
		m.centered(screen, r, "No notes in "+m.th.value.Bold(true).Render(m.project.Name)+" yet.",
			m.th.faint.Render("A note holds a thought that isn't a task yet."),
			m.th.faint.Render("Press ")+m.th.keyBold.Render("n")+m.th.faint.Render(" to add one."))
		return
	case len(notes) == 0 && len(m.notes) > 0 && m.mode != modeInput:
		m.centered(screen, r, "No notes match "+m.th.value.Bold(true).Render(m.noteFilter)+".",
			m.th.keyBold.Render("Esc")+m.th.faint.Render(" clears the filter."))
		return
	}
	main := r
	if m.noteOpen {
		if !m.wide() {
			m.notePane(screen, r)
			return
		}
		dw := m.detailWidth()
		main.w = r.w - dw - 1
		m.notePane(screen, rect{r.x + main.w + 1, r.y, dw, r.h})
	}
	m.noteGrid(screen, main, notes)
}

func (m *model) noteGrid(screen []string, r rect, notes []core.Note) {
	cols := m.noteCols()
	cardH := notePreview + 2
	rows := (len(notes) + cols - 1) / cols
	fit := max(1, r.h/cardH)
	if rows > fit {
		fit = max(1, (r.h-1)/cardH) // room for "↓ N more"
	}
	sel := m.noteRow / cols
	m.noteTop = max(0, min(m.noteTop, sel, rows-fit))
	if sel >= m.noteTop+fit {
		m.noteTop = sel - fit + 1
	}
	cardW, extra := (r.w-(cols-1))/cols, (r.w-(cols-1))%cols
	for i := m.noteTop * cols; i < len(notes) && i < (m.noteTop+fit)*cols; i++ {
		c := i % cols
		x := r.x + c*(cardW+1) + min(c, extra)
		w := cardW
		if c < extra {
			w++
		}
		y := r.y + (i/cols-m.noteTop)*cardH
		focused := i == m.noteRow && !(m.noteOpen && m.focusNote)
		place(screen, x, y, m.noteCard(notes[i], w, focused))
		m.onClick(rect{x, y, w, cardH}, func() tea.Cmd {
			m.noteRow = i
			m.noteSelChanged()
			m.openNote(notes[i])
			return nil
		})
	}
	if after := len(notes) - (m.noteTop+fit)*cols; after > 0 {
		place(screen, r.x, r.y+fit*cardH, []string{m.th.faint.Render(fmt.Sprintf(" ↓ %d more", after))})
	}
}

// noteCard draws a note's card width cells wide: the start of the note, and
// on the bottom border who wrote it, when, and how many lines don't show.
func (m *model) noteCard(n core.Note, width int, selected bool) []string {
	th := m.th
	if selected && !m.dimmed {
		th = th.onSelection()
	}
	tw := width - 4
	all := noteLines(n.Body, tw, th)
	shown := all[:min(len(all), notePreview)]
	var lines []string
	for _, l := range shown {
		lines = append(lines, th.plain.Render(" ")+fitIn(l, tw, th.plain)+th.plain.Render(" "))
	}
	for len(lines) < notePreview {
		lines = append(lines, th.plain.Render(strings.Repeat(" ", width-2)))
	}
	o := boxOpts{keys: m.noteByline(n, timeAgo(n.CreatedAt)), focused: selected}
	if hidden := len(all) - len(shown); hidden > 0 {
		o.bottom = m.th.faint.Render(fmt.Sprintf("+%d %s", hidden, plural(hidden, "line", "lines")))
	}
	if m.mode == modeConfirm && m.confirm.note == n.Number && !m.noteOpen {
		o = boxOpts{keys: m.confirm.question, alert: true}
	}
	return m.box(width, notePreview+2, o, lines)
}

// noteByline is who wrote a note and when: "Claude · 2 h ago", or just the
// time for a note without an agent.
func (m *model) noteByline(n core.Note, when string) string {
	if n.Run == nil {
		return m.th.faint.Render(when)
	}
	return m.th.value.Render(core.AgentName(n.Run.Agent)) + m.th.faint.Render(" · "+when)
}

// noteLines lays out a note's text width cells wide as written, not as
// rendered Markdown: prose wraps, "> " quotes wrap under their marker, and
// code between fences is cut, not wrapped, and shown in the value color.
func noteLines(body string, width int, th theme) []string {
	var lines []string
	inCode := false
	for _, raw := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~"):
			inCode = !inCode
			lines = append(lines, th.faint.Render(truncate(raw, width)))
		case inCode:
			lines = append(lines, th.value.Render(truncate(strings.ReplaceAll(raw, "\t", "    "), width)))
		case trimmed == "":
			lines = append(lines, "")
		case strings.HasPrefix(trimmed, ">"):
			for i, l := range wrapWords(strings.TrimSpace(strings.TrimPrefix(trimmed, ">")), max(1, width-2)) {
				lead := th.faint.Render("> ")
				if i > 0 {
					lead = th.plain.Render("  ")
				}
				lines = append(lines, lead+th.plain.Render(l))
			}
		default:
			for _, l := range wrapWords(raw, width) {
				lines = append(lines, th.plain.Render(l))
			}
		}
	}
	return lines
}

// The note pane.

func (m *model) notePane(screen []string, r rect) {
	n, ok := m.note(m.noteNum)
	if !ok {
		return
	}
	m.noteRect = r
	th := m.th
	textW := r.w - 4
	focused := m.focusNote || !m.wide()

	var content []string
	field := func(name, v string) {
		content = append(content, th.faint.Render(fmt.Sprintf("%-9s", name))+v)
	}
	if n.Run != nil {
		field("author", th.value.Render(n.Run.String()))
	}
	field("created", formatTime(n.CreatedAt))
	if n.Edited() {
		field("updated", formatTime(n.UpdatedAt))
	}
	content = append(content, "")
	content = append(content, noteLines(n.Body, textW, th)...)

	actions := th.keyBold.Render("t make task") + th.faint.Render(" · ") + th.keys("e", "edit") +
		th.faint.Render(" · ") + th.keys("d", "delete") + th.faint.Render(" · ") + th.keys("y", "copy text")
	areaH := r.h - 2 - 2
	maxScroll := max(0, len(content)-areaH)
	m.noteScroll = max(0, min(m.noteScroll, maxScroll))
	var body []string
	for i := m.noteScroll; i < len(content) && i < m.noteScroll+areaH; i++ {
		body = append(body, " "+content[i])
	}
	for len(body) < areaH {
		body = append(body, "")
	}
	body = append(body, " "+th.faint.Render(strings.Repeat("─", textW)), " "+actions)

	m.onClick(r, func() tea.Cmd {
		m.focusNote = true
		return nil
	})
	x := r.x + 2
	for _, a := range [][2]string{{"t", "t make task"}, {"e", "e edit"}, {"d", "d delete"}, {"y", "y copy text"}} {
		w := lipgloss.Width(a[1])
		m.onKey(rect{x, r.y + r.h - 2, w, 1}, a[0])
		x += w + 3
	}
	back := th.keys("esc", "close")
	m.onKey(rect{r.x + r.w - 2 - lipgloss.Width(back), r.y, lipgloss.Width(back), 1}, "esc")

	scroll := ""
	switch {
	case maxScroll == 0:
	case m.noteScroll == 0:
		scroll = th.faint.Render("↓ ") + th.keys("j k", "scroll")
	case m.noteScroll == maxScroll:
		scroll = th.faint.Render("↑ ") + th.keys("j k", "scroll")
	default:
		scroll = th.faint.Render("↕ ") + th.keys("j k", "scroll")
	}
	o := boxOpts{title: th.keyBold.Render("Note") + th.faint.Render(" · "+timeAgo(n.CreatedAt)), right: back, bottom: scroll, focused: focused}
	if m.mode == modeConfirm && m.confirm.note == n.Number {
		o = boxOpts{title: o.title, keys: m.confirm.question + "  " + th.questionKeys("y", m.confirm.yes, "n", m.confirm.no), alert: true}
	}
	place(screen, r.x, r.y, m.box(r.w, r.h, o, body))
}

// The note box.

type noteEditedMsg struct {
	draft string
	err   error
}

// startNoteBox opens the box for a new note, or to edit n.
func (m *model) startNoteBox(n core.Note) tea.Cmd {
	if !m.wide() {
		m.noteOpen = false
	}
	m.mode, m.editNote, m.noteOrig = modeNoteBox, n.Number, n.Body
	m.area.SetWidth(m.modalWidth(90) - 8)
	m.area.SetHeight(m.h)
	m.area.SetValue(n.Body)
	return m.area.Focus()
}

// writingNote reports whether the note box is open, also while asking
// whether to discard it.
func (m *model) writingNote() bool {
	return m.mode == modeNoteBox || (m.mode == modeConfirm && m.confirm.back == modeNoteBox)
}

func (m *model) noteBoxKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.noteText() == m.noteOrig {
			m.endNoteBox()
			return nil
		}
		q := "Discard this note?"
		if m.editNote > 0 {
			q = "Discard your changes to this note?"
		}
		m.confirm = confirm{
			question: m.th.err.Bold(true).Render(q),
			yes:      "discard", no: "keep writing", back: modeNoteBox,
			onYes: func() tea.Cmd {
				m.endNoteBox()
				return nil
			},
			onNo: func() tea.Cmd { return m.area.Focus() },
		}
		m.mode = modeConfirm
		m.area.Blur()
		return nil
	case "ctrl+s":
		return m.saveNote(m.noteText())
	case "ctrl+g":
		return m.openNoteEditor(m.noteText())
	}
	var cmd tea.Cmd
	m.area, cmd = m.area.Update(msg)
	return cmd
}

// openNoteEditor moves the note box's text to the editor; the note is saved
// when the editor exits.
func (m *model) openNoteEditor(text string) tea.Cmd {
	draft, err := editor.Draft(m.project.Name+"-note-*.md", text)
	if err != nil {
		m.fail(err)
		return nil
	}
	return tea.ExecProcess(editor.Command(draft), func(err error) tea.Msg { return noteEditedMsg{draft, err} })
}

// editNoteInEditor opens note n in the editor, as the note box's ctrl-g
// does.
func (m *model) editNoteInEditor(n core.Note) tea.Cmd {
	m.editNote, m.noteOrig = n.Number, n.Body
	return m.openNoteEditor(n.Body)
}

// noteText is the note box's text, with the tabs of the note being edited
// put back.
func (m *model) noteText() string {
	return restoreTabs(m.area.Value(), m.noteOrig)
}

func (m *model) endNoteBox() {
	m.mode = modeNormal
	m.area.Blur()
}

func (m *model) noteEdited(msg noteEditedMsg) tea.Cmd {
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
	return m.saveNote(string(src))
}

func (m *model) saveNote(text string) tea.Cmd {
	body, err := core.CleanNoteBody(text)
	if err != nil && m.mode != modeNoteBox {
		// From the editor after the lane's ctrl-g: no box to go back to.
		m.msg = errMsg(m.th, "The note is empty, so it wasn't changed.")
		return nil
	}
	if err != nil {
		m.msg = errMsg(m.th, "The note is empty. Write something, or press esc to cancel.")
		return nil
	}
	if m.editNote > 0 && body == m.noteOrig {
		m.endNoteBox()
		m.msg = message{[]string{"No changes to the note"}}
		return nil
	}
	if m.editNote == 0 {
		n, err := m.s.AddNote(m.ctx, m.project.Name, body, "")
		if err != nil {
			m.fail(err)
			return nil
		}
		m.selNote, m.noteRow = n.Number, 0
		m.msg = okMsg(m.th, "Added a note")
	} else {
		if _, err := m.s.UpdateNote(m.ctx, m.project.Name, m.editNote, body); err != nil {
			m.fail(err)
			return nil
		}
		m.msg = okMsg(m.th, "Updated the note")
	}
	m.endNoteBox()
	m.reload()
	return nil
}

// drawNoteBox draws the note box in the middle of r.
func (m *model) drawNoteBox(screen []string, r rect) {
	th := m.th
	title := th.keyBold.Render("New note") + th.faint.Render(" in "+m.project.Name)
	if m.editNote > 0 {
		n, _ := m.note(m.editNote)
		title = th.keyBold.Render("Edit note") + th.faint.Render(" · "+timeAgo(n.CreatedAt))
	}
	w := m.modalWidth(90)
	textW := w - 4
	alert := m.mode == modeConfirm
	keyText := th.keys("ctrl-s", "save", "ctrl-g", "open in "+editor.Name(), "esc", "cancel")
	if alert {
		keyText = m.confirm.question + "  " + th.questionKeys("y", m.confirm.yes, "n", m.confirm.no)
		title = th.err.Bold(true).Render(ansi.Strip(title)) // the whole frame asks
	}
	bottom, keys := boxKeys(w, keyText)
	areaH := max(3, min(10, min(r.h, 30)-2-2-len(keys)))
	m.area.SetWidth(textW - 4)
	m.area.SetHeight(areaH)
	var area []string
	for _, l := range strings.Split(m.area.View(), "\n") {
		area = append(area, " "+l)
	}
	var body []string
	for _, l := range m.box(textW, areaH+2, boxOpts{focused: !alert}, area) {
		body = append(body, " "+l)
	}
	body = append(body, keys...)
	h := len(body) + 2
	x, y := r.x+(r.w-w)/2, r.y+(r.h-h)/2
	m.onClick(rect{0, 0, m.w, m.h}, func() tea.Cmd { return nil }) // clicks behind it do nothing
	place(screen, x, y, m.box(w, h, boxOpts{title: title, keys: bottom, focused: true, alert: alert}, body))
}

// Making a task from a note, in the edit box.

// startTaskFromNote opens the edit box for a new backlog task with the note
// as its description and no title.
func (m *model) startTaskFromNote(n core.Note) tea.Cmd {
	m.fromNote = n.Number
	m.mode, m.editTitle, m.editRef, m.editBase = modeEditTask, true, "", core.Task{Description: n.Body}
	m.titleIn.SetValue("")
	m.area.SetWidth(m.modalWidth(110) - 8)
	m.area.SetHeight(m.h)
	m.area.SetValue(n.Body)
	m.area.MoveToBegin()
	m.area.Blur()
	return m.titleIn.Focus()
}

// saveTaskFromNote adds the task and removes the note. An invalid task keeps
// the box open with the error shown.
func (m *model) saveTaskFromNote() tea.Cmd {
	patch := m.taskEditPatch()
	if patch.Title == nil { // the title starts empty, so an unchanged one is empty
		m.msg = errMsg(m.th, "Write a title for the task, or press esc to cancel.")
		return nil
	}
	t := core.Task{Title: *patch.Title, Description: m.editBase.Description, Status: core.StatusBacklog, Priority: core.PriorityNone}
	if patch.Description != nil {
		t.Description = *patch.Description
	}
	t, err := m.s.TaskFromNote(m.ctx, m.project.Name, m.fromNote, t)
	if err != nil {
		m.msg = errMsg(m.th, "The task wasn't added: "+errText(err))
		return nil
	}
	m.endEditTask()
	m.reload()
	m.msg = okMsg(m.th, "Added "+m.th.value.Render(t.Ref)+": "+t.Title)
	return nil
}

// fromNoteHint is the line under the description when making a task from a
// note: what saving does.
func (m *model) fromNoteHint() string {
	return m.th.faint.Render("Saving adds the task to ") + m.th.bold.Render("backlog") + m.th.faint.Render(" and removes the note.")
}

// The board's notes lane.

// laneShown reports whether the board shows the notes lane: N turned it on,
// and archived tasks aren't shown, since notes aren't archived.
func (m *model) laneShown() bool {
	return m.showLane && !m.showArchived && m.view == viewBoard
}

// laneKey handles keys while the selection is in the board's notes lane.
func (m *model) laneKey(k string) tea.Cmd {
	if m.noteFocused() {
		return m.notePaneKey(k)
	}
	switch k {
	case "l", "right":
		m.inLane, m.noteOpen = false, false
		m.selectRef(m.selRef)
	case "h", "left":
	case "j", "down":
		m.moveNote(1)
	case "k", "up":
		m.moveNote(-1)
	case "g":
		m.moveNote(-1 << 30)
	case "G":
		m.moveNote(1 << 30)
	case "enter":
		if n, ok := m.selectedNote(); ok {
			m.openNote(n)
		}
	case "tab":
		if m.noteOpen {
			m.focusNote = true
		}
	case "esc":
		m.noteOpen = false
	default:
		return m.noteKey(k)
	}
	return nil
}

// drawLane draws the notes lane in r: each note's first line, and who wrote
// it and when, newest first.
func (m *model) drawLane(screen []string, r rect) {
	notes := m.shownNotes()
	focused := m.inLane && !(m.noteOpen && m.focusNote)
	name := m.th.bold.Render("notes")
	if focused {
		name = m.th.keyBold.Render("notes")
	}
	title := name + " " + m.th.faint.Render(fmt.Sprint(len(notes)))
	m.onClick(r, func() tea.Cmd {
		m.inLane, m.detailOpen, m.focusNote = true, false, false
		m.selectNote(m.selNote)
		return nil
	})
	inner, avail := r.w-2, r.h-2
	var body []string
	if m.mode == modeInput && m.inputKind == inputNewNote {
		m.input.SetWidth(inner - 5)
		body = m.box(inner, 3, boxOpts{title: m.th.faint.Render("new note"), focused: true}, []string{" " + m.input.View()})
		avail -= 3
	}
	// Each note takes two lines and a blank one after it; the last blank
	// line can be cut.
	fit := max(1, (avail+1)/3)
	if fit < len(notes) {
		fit = max(1, avail/3) // room for "↓ N more"
	}
	m.laneTop = max(0, min(m.laneTop, m.noteRow, len(notes)-fit))
	if m.noteRow >= m.laneTop+fit {
		m.laneTop = m.noteRow - fit + 1
	}
	if len(notes) == 0 {
		place(screen, r.x, r.y, m.box(r.w, r.h, boxOpts{title: title, focused: focused, dashed: true}, append(body, m.th.faint.Render(" —"))))
		return
	}
	for i := m.laneTop; i < len(notes) && i < m.laneTop+fit; i++ {
		n := notes[i]
		th, lead, text := m.th, " ", m.th.plain
		if i == m.noteRow && m.inLane {
			th = m.th.onSelection()
			lead, text = m.th.focus.Render("┃"), th.bold // the weight of a selected card's border
		}
		first, _, _ := strings.Cut(strings.TrimLeft(n.Body, "\n"), "\n")
		when := timeAgo(n.CreatedAt)
		if extra := strings.Count(n.Body, "\n"); extra > 0 { // the lines after the first

			when += fmt.Sprintf(" · +%d %s", extra, plural(extra, "line", "lines"))
		}
		byline := th.faint.Render(when)
		if n.Run != nil {
			byline = th.value.Render(core.AgentName(n.Run.Agent)) + th.faint.Render(" · "+when)
		}
		y := r.y + 1 + len(body)
		body = append(body, lead+fitIn(text.Render(truncate(first, inner-1)), inner-1, th.plain),
			lead+fitIn(byline, inner-1, th.plain), "")
		m.onClick(rect{r.x + 1, y, inner, 2}, func() tea.Cmd {
			m.inLane, m.noteRow = true, i
			m.noteSelChanged()
			m.openNote(n)
			return nil
		})
	}
	if after := len(notes) - m.laneTop - fit; after > 0 {
		body = append(body[:len(body)-1], m.th.faint.Render(fmt.Sprintf(" ↓ %d more", after)))
	}
	place(screen, r.x, r.y, m.box(r.w, r.h, boxOpts{title: title, focused: focused, dashed: true}, body))
}
