// Package tui is Taskship's full-screen terminal UI: a board and a list of a
// project's tasks, a detail view, and the actions the CLI has for tasks.
// It holds no domain logic: it reads and changes tasks through the store and
// launches agents through dispatch, like the CLI.
package tui

import (
	"context"
	"errors"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/dispatch"
	"github.com/jhdenham/taskship/internal/store"
)

// Minimum terminal size.
const minWidth, minHeight = 80, 24

// wideWidth is the width from which the detail pane sits beside the board
// or list instead of replacing it.
const wideWidth = 140

// Options says where the TUI starts.
type Options struct {
	Project  string // the current folder's project; "" opens the project picker
	Folder   string // the current folder, as shown to the user ("~/code/myproject")
	CanSetUp bool   // whether "taskship init" can set up Folder
}

// Outcome is what the user chose to do after leaving the TUI.
type Outcome struct {
	Launch *dispatch.Plan // a Ready agent run to start
	SetUp  bool           // run init in the current folder, then reopen
}

// Run shows the TUI until the user quits, launches an agent, or asks to set
// up the current folder.
func Run(ctx context.Context, s *store.Store, opts Options) (Outcome, error) {
	m := newModel(ctx, s, opts)
	_, err := tea.NewProgram(m, tea.WithContext(ctx)).Run()
	return m.outcome, err
}

type viewKind int

const (
	viewBoard viewKind = iota
	viewList
)

// mode is what keys currently go to.
type mode int

const (
	modeNormal    mode = iota
	modeHelp           // the help overlay
	modeProjects       // the project picker
	modeMenu           // a status or priority menu
	modeInput          // the filter, labels or quick-add input
	modeComment        // writing a comment
	modeConfirm        // a y/n question
	modeEditError      // an invalid edit, waiting for e or esc
	modeEditTask       // editing the title and description in the detail pane
)

type inputKind int

const (
	inputFilter inputKind = iota
	inputLabels
	inputNew
)

// message is the status line above the footer: a first line and optional
// more lines (hints, the rest of an error).
type message struct {
	lines []string
}

type menu struct {
	field   string // "status" or "priority": where the menu opens
	title   string
	options []string // shown
	cursor  int
	current int
	pick    func(i int) tea.Cmd
}

type confirm struct {
	question string // styled
	yes, no  string // "archive", "keep"
	onYes    func() tea.Cmd
	comment  int            // the comment being deleted, for the detail view
	back     mode           // the mode to return to on no
	onNo     func() tea.Cmd // run on no, if set
}

type projectRow struct {
	name, prefix string
	open         int
}

type model struct {
	ctx     context.Context
	s       *store.Store
	opts    Options
	outcome Outcome

	w, h  int
	th    theme
	zones []zone
	// anchors are where the last render drew the field values of the detail
	// pane ("field:status"), which open their menus when clicked.
	anchors map[string]rect

	// Projects, for the picker. hasProjects is false only when there are
	// none at all.
	projects    []projectRow
	projCursor  int
	projTop     int // the first project the picker shows
	hasProjects bool

	project core.Project // zero until one is open
	tasks   []core.Task  // all of the project's tasks, archived included

	view          viewKind
	showCanceled  bool
	showArchived  bool
	filter        filter
	sortBy        int
	col           int
	rows          map[core.Status]int // selected row per board column
	listRow       int
	selRef        string // the selected task, kept across reloads
	quickAddCol   core.Status
	listTop       int
	colTop        map[core.Status]int
	detailOpen    bool
	focusDetail   bool
	detailRef     string
	comments      []core.Comment
	detailScroll  int
	pick          int // picked comment index, -1 for none
	scrollToPick  bool
	editComment   int // number of the comment being edited, 0 for a new one
	editDraft     string
	editDraftFor  string
	detailRect    rect
	inputRef      string
	editRef       string         // the task an edit or comment box is for
	commentNumber int            // the number a new comment will likely get
	earlier       []core.Comment // the comments shown above the comment box, oldest first
	showEarlier   bool           // the comment box shows them (ctrl-r; off whenever a box opens)
	commentOrig   string         // the comment box's text when it opened
	dimmed        bool           // drawing what is behind a box: no focus highlights
	fieldLines    map[string]int // detail content line of each field
	fieldX        int            // where field values start in the detail content
	prevFilter    filter
	filterErr     string

	mode      mode
	input     textinput.Model
	inputKind inputKind
	area      textarea.Model
	titleIn   textinput.Model // the title while editing a task
	editTitle bool            // the title field has the focus, not the description
	menu      menu
	confirm   confirm
	msg       message
}

func newModel(ctx context.Context, s *store.Store, opts Options) *model {
	m := &model{ctx: ctx, s: s, opts: opts, th: newTheme(true), pick: -1,
		rows: map[core.Status]int{}, colTop: map[core.Status]int{}}
	m.input = textinput.New()
	m.input.Prompt = ""
	m.area = textarea.New()
	m.area.ShowLineNumbers = false
	m.area.Prompt = ""
	m.area.MaxHeight = 0 // no limit on lines (the default stops enter at 99)
	m.titleIn = textinput.New()
	m.titleIn.Prompt = ""
	m.loadProjects()
	switch {
	case opts.Project != "":
		m.openProject(opts.Project)
	case m.hasProjects:
		m.openPicker()
	}
	return m
}

func (m *model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tea.BackgroundColorMsg:
		m.th = newTheme(msg.IsDark())
		return m, nil
	case tea.KeyPressMsg:
		return m, m.keyPress(msg)
	case tea.PasteMsg:
		return m, m.forwardToInput(msg)
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return m, nil
		}
		for i := len(m.zones) - 1; i >= 0; i-- {
			if m.zones[i].r.has(msg.X, msg.Y) {
				return m, m.zones[i].click()
			}
		}
		return m, nil
	case tea.MouseWheelMsg:
		return m, m.wheel(msg)
	case editedMsg:
		return m, m.edited(msg)
	case commentEditedMsg:
		return m, m.commentEdited(msg)
	}
	return m, m.forwardToInput(msg)
}

// forwardToInput passes other messages (cursor blinks, pastes) to the active
// input.
func (m *model) forwardToInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.mode {
	case modeInput:
		m.input, cmd = m.input.Update(msg)
		if _, ok := msg.(tea.PasteMsg); ok {
			m.inputChanged() // only input changes the filter, not cursor blinks
		}
	case modeComment:
		m.area, cmd = m.area.Update(msg)
	case modeEditTask:
		if m.editTitle {
			m.titleIn, cmd = m.titleIn.Update(msg)
		} else {
			m.area, cmd = m.area.Update(msg)
		}
	}
	return cmd
}

// keyPress handles a key. Text inputs get their keys first.
func (m *model) keyPress(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if k == "ctrl+c" {
		m.cleanUpDraft()
		return tea.Quit
	}
	switch m.mode {
	case modeInput:
		return m.inputKey(msg)
	case modeComment:
		return m.commentKey(msg)
	case modeEditTask:
		return m.editTaskKey(msg)
	}
	return m.key(k)
}

// key handles a key outside text inputs. Clicks on keys in footers and
// action lines come here too.
func (m *model) key(k string) tea.Cmd {
	if m.mode == modeNormal || m.mode == modeProjects {
		m.msg = message{}
	}
	switch m.mode {
	case modeHelp:
		if k == "esc" || k == "?" || k == "q" {
			m.mode = modeNormal
			if m.project.ID == "" && m.hasProjects {
				m.mode = modeProjects
			}
		}
		return nil
	case modeProjects:
		return m.projectsKey(k)
	case modeMenu:
		return m.menuKey(k)
	case modeConfirm:
		return m.confirmKey(k)
	case modeEditError:
		return m.editErrorKey(k)
	case modeInput, modeComment, modeEditTask:
		return nil
	}
	if m.project.ID == "" {
		return m.emptyKey(k)
	}
	switch k {
	case "q":
		return tea.Quit
	case "?":
		m.mode = modeHelp
		return nil
	case "P":
		m.openPicker()
		return nil
	case "1":
		m.view = viewBoard
		m.selectRef(m.selRef)
		return nil
	case "2":
		m.view = viewList
		m.selectRef(m.selRef)
		return nil
	case "/":
		m.startInput(inputFilter, m.filter.text, "")
		return nil
	case "C":
		m.showCanceled = !m.showCanceled
		m.selectRef(m.selRef)
		return nil
	case "A":
		m.showArchived = !m.showArchived
		m.detailOpen = false
		m.selectRef(m.selRef)
		return nil
	case "R":
		m.reload()
		return nil
	case "n":
		return m.startQuickAdd()
	}
	if m.detailFocused() {
		return m.detailKey(k)
	}
	if k == "esc" {
		switch {
		case m.detailOpen:
			m.detailOpen = false
		case m.filter.active():
			m.filter = filter{}
			m.selectRef(m.selRef)
		}
		return nil
	}
	if m.view == viewBoard {
		if cmd, ok := m.boardKey(k); ok {
			return cmd
		}
	} else if cmd, ok := m.listKey(k); ok {
		return cmd
	}
	if k == "tab" && m.detailOpen {
		m.focusDetail = true
		return nil
	}
	return m.taskKey(k)
}

// taskKey handles the keys that act on the selected task, in any view.
func (m *model) taskKey(k string) tea.Cmd {
	t, ok := m.target()
	if !ok {
		return nil
	}
	switch k {
	case "enter":
		m.openDetail(t.Ref)
	case "s":
		m.statusMenu(t)
	case "p":
		m.priorityMenu(t)
	case "#":
		m.startInput(inputLabels, strings.Join(t.Labels, ", "), t.Ref)
	case "a":
		m.askArchive(t)
	case "e":
		return m.startEditTask(t)
	case "y":
		m.msg = okMsg(m.th, "Copied "+t.Ref)
		return tea.SetClipboard(t.Ref)
	case "c":
		return m.startComment(t, 0, "")
	case "r":
		return m.launch(t)
	}
	return nil
}

// emptyKey handles keys with no project open: no projects at all.
func (m *model) emptyKey(k string) tea.Cmd {
	switch k {
	case "q":
		return tea.Quit
	case "?":
		m.mode = modeHelp
	case "i":
		if m.opts.CanSetUp {
			m.outcome.SetUp = true
			return tea.Quit
		}
	}
	return nil
}

// detailFocused reports whether keys go to the detail pane: it has the
// focus beside the board, or it replaces the board.
func (m *model) detailFocused() bool {
	return m.detailOpen && (m.focusDetail || !m.wide())
}

// wide reports whether the detail pane goes beside the board or list.
func (m *model) wide() bool { return m.w >= wideWidth }

// loadProjects reads the projects and their open task counts for the
// picker.
func (m *model) loadProjects() {
	ps, err := m.s.ListProjects(m.ctx, false)
	if err != nil {
		m.fail(err)
		return
	}
	m.hasProjects = len(ps) > 0
	tasks, err := m.s.ListTasks(m.ctx, store.TaskFilter{})
	if err != nil {
		m.fail(err)
		return
	}
	open := map[string]int{}
	for _, t := range tasks {
		if t.Status != core.StatusDone && t.Status != core.StatusCanceled {
			open[t.Project]++
		}
	}
	m.projects = m.projects[:0]
	for _, p := range ps {
		m.projects = append(m.projects, projectRow{p.Name, p.TaskPrefix, open[p.Name]})
	}
}

// openProject switches to a project and resets the views.
func (m *model) openProject(name string) {
	p, err := m.s.GetProject(m.ctx, name)
	if err != nil {
		m.fail(err)
		return
	}
	m.project = p
	m.filter, m.showArchived, m.showCanceled = filter{}, false, false
	m.detailOpen, m.focusDetail, m.selRef = false, false, ""
	m.col, m.listRow, m.listTop = 0, 0, 0
	clear(m.rows)
	clear(m.colTop)
	m.reload()
}

// reload reads the project's tasks again, keeping the selection on the same
// task where it still shows.
func (m *model) reload() {
	tasks, err := m.s.ListTasks(m.ctx, store.TaskFilter{Project: m.project.Name, IncludeArchived: true})
	if err != nil {
		m.fail(err)
		return
	}
	m.tasks = tasks
	if m.detailOpen {
		if _, ok := m.task(m.detailRef); !ok {
			m.detailOpen = false
		}
	}
	m.selectRef(m.selRef)
	if m.detailOpen {
		m.loadComments()
	}
}

// task returns the loaded task with ref.
func (m *model) task(ref string) (core.Task, bool) {
	i := slices.IndexFunc(m.tasks, func(t core.Task) bool { return t.Ref == ref })
	if i < 0 {
		return core.Task{}, false
	}
	return m.tasks[i], true
}

// target is the task actions apply to: the one in the detail view when it
// is open, else the selected one.
func (m *model) target() (core.Task, bool) {
	if m.detailOpen {
		return m.task(m.detailRef)
	}
	return m.selected()
}

// fail shows err in the status line.
func (m *model) fail(err error) {
	text := errText(err)
	m.msg = errMsg(m.th, strings.ToUpper(text[:1])+text[1:])
}

// errText is an error's message without the "invalid input: " prefix.
func errText(err error) string {
	s := err.Error()
	if errors.Is(err, core.ErrInvalid) {
		s = strings.TrimPrefix(s, core.ErrInvalid.Error()+": ")
	}
	if s == "" {
		s = "unknown error"
	}
	return s
}

func okMsg(th theme, text string, more ...string) message {
	return message{append([]string{th.ok.Render("✓") + " " + text}, more...)}
}

func errMsg(th theme, text string, more ...string) message {
	lines := []string{th.err.Render("✗ " + text)}
	for _, l := range more {
		lines = append(lines, "  "+l)
	}
	return message{lines}
}
