package tui

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

var update = flag.Bool("update", false, "rewrite golden files")

func init() {
	formatTime = func(time.Time) string { return "Nov 11, 09:20" }
	// Screens name the editor; keep them the same everywhere.
	os.Setenv("VISUAL", "")
	os.Setenv("EDITOR", "vi")
}

const habitDescription = `Show each habit as a row and the days of the current week as columns.
Clicking a cell toggles it.

## Acceptance criteria

- [x] Weekly view shows each habit
- [ ] Data persists across restarts
- [ ] Works at 80 columns`

// sample is the design brief's sample data, numbered PROJ-1 to PROJ-9.
var sample = []core.Task{
	{Title: "Upgrade to Postgres 16", Status: core.StatusDone, Priority: core.PriorityHigh, Labels: []string{"infra"}},
	{Title: "Keyboard help overlay", Status: core.StatusDone, Priority: core.PriorityLow, Labels: []string{"cli"}},
	{Title: "Document the export format", Status: core.StatusBacklog, Priority: core.PriorityLow, Labels: []string{"docs"}},
	{Title: "Try a compact board layout", Status: core.StatusBacklog, Priority: core.PriorityNone},
	{Title: "Rate-limit webhook retries", Status: core.StatusTodo, Priority: core.PriorityHigh, Labels: []string{"api", "bug"}},
	{Title: "Add --json to the status command", Status: core.StatusTodo, Priority: core.PriorityMedium, Labels: []string{"cli"}},
	{Title: "Rename the sync settings", Status: core.StatusTodo, Priority: core.PriorityLow},
	{Title: "Fix flaky login test", Status: core.StatusInProgress, Priority: core.PriorityUrgent, Labels: []string{"bug"}},
	{Title: "Weekly habit tracker view", Status: core.StatusInProgress, Priority: core.PriorityMedium, Labels: []string{"web"}, Description: habitDescription},
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seeded returns a store with myproject and the sample tasks, and two
// comments on PROJ-9, the second one edited.
func seeded(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	if _, err := s.CreateProject(ctx, core.Project{Name: "myproject", TaskPrefix: "PROJ"}); err != nil {
		t.Fatal(err)
	}
	for _, task := range sample {
		if _, err := s.CreateTask(ctx, "myproject", task); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{"Started with a plain table; a grid component felt like overkill.", "Persistence blocked."} {
		if _, err := s.AddComment(ctx, "PROJ-9", body, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.UpdateComment(ctx, "PROJ-9", 2, "Persistence blocked on the storage refactor in PROJ-6."); err != nil {
		t.Fatal(err)
	}
	return s
}

func open(t *testing.T, s *store.Store, opts Options, w, h int) *model {
	t.Helper()
	m := newModel(context.Background(), s, opts)
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func myproject(t *testing.T, w, h int) (*model, *store.Store) {
	s := seeded(t)
	return open(t, s, Options{Project: "myproject", Folder: "~/code/myproject", CanSetUp: true}, w, h), s
}

// press sends keys by name: "j", "enter", "ctrl+s"; "text:abc" types abc.
func press(m *model, keys ...string) {
	for _, k := range keys {
		if text, ok := strings.CutPrefix(k, "text:"); ok {
			for _, r := range text {
				m.Update(keyMsg(string(r)))
			}
			continue
		}
		m.Update(keyMsg(k))
	}
}

// screen renders m as plain text without trailing spaces.
func screen(m *model) string {
	lines := strings.Split(ansi.Strip(m.render()), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("screen %s differs from %s; got:\n%s", name, path, got)
	}
}

func task(t *testing.T, s *store.Store, ref string) core.Task {
	t.Helper()
	got, err := s.GetTask(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// The golden screens correspond to the mockups' artboards.
func TestScreens(t *testing.T) {
	for _, tt := range []struct {
		name string
		w, h int
		keys []string
	}{
		{"board-80", 80, 24, nil},
		{"board-180", 180, 44, nil},
		{"list-80", 80, 24, []string{"2", "j", "j", "j"}},
		{"list-180", 180, 44, []string{"2"}},
		{"detail-80", 80, 24, []string{"l", "l", "j", "enter"}},
		{"detail-completed-80", 80, 24, []string{"l", "l", "l", "enter"}},
		{"detail-scrolled-80", 80, 24, []string{"l", "l", "j", "enter", "G"}},
		{"board-detail-180", 180, 44, []string{"l", "l", "j", "enter"}},
		{"list-detail-180", 180, 44, []string{"2", "j", "j", "enter"}},
		{"help-80", 80, 24, []string{"?"}},
		{"help-180", 180, 44, []string{"?"}},
		{"filter-typing-80", 80, 24, []string{"/", "text:habit"}},
		{"filter-label-180", 180, 44, []string{"/", "text:#bug", "enter"}},
		{"filter-label-menu-180", 180, 44, []string{"/", "text:#bug", "enter", "l", "s"}},
		{"filter-none-80", 80, 24, []string{"/", "text:habits", "enter"}},
		{"status-menu-80", 80, 24, []string{"l", "s"}},
		{"labels-input-80", 80, 24, []string{"l", "#"}},
		{"priority-menu-detail-80", 80, 24, []string{"l", "l", "j", "enter", "p"}},
		{"status-menu-detail-180", 180, 44, []string{"l", "l", "j", "enter", "s"}},
		{"status-menu-list-80", 80, 24, []string{"2", "g", "s"}},
		{"priority-menu-list-bottom-80", 80, 24, []string{"2", "G", "p"}},
		{"labels-detail-80", 80, 24, []string{"l", "enter", "#"}},
		{"quick-add-80", 80, 24, []string{"l", "n", "text:Update README"}},
		{"archive-confirm-80", 80, 24, []string{"l", "j", "j", "a"}},
		{"archived-80", 80, 24, []string{"l", "j", "j", "a", "y", "2"}},
		{"archived-empty-80", 80, 24, []string{"A"}},
		{"comment-new-80", 80, 24, []string{"l", "l", "j", "enter", "c", "text:Picking it back up."}},
		{"comment-delete-80", 80, 24, []string{"l", "l", "j", "enter", "]", "]", "d"}},
		{"edit-task-80", 80, 24, []string{"l", "l", "j", "e"}},
		{"edit-task-180", 180, 44, []string{"l", "l", "j", "e", "tab"}},
		{"edit-task-board-180", 180, 44, []string{"l", "e"}},
		{"comment-board-80", 80, 24, []string{"l", "c", "text:Looks fine."}},
		{"comment-new-180", 180, 44, []string{"l", "l", "j", "c", "ctrl+r"}},
		{"comment-new-shown-80", 80, 24, []string{"l", "l", "j", "enter", "c", "ctrl+r", "text:Picking it back up."}},
		{"comment-edit-80", 80, 24, []string{"l", "l", "j", "enter", "]", "]", "e"}},
		{"edit-task-discard-80", 80, 24, []string{"l", "l", "j", "e", "text:!", "esc"}},
		{"comment-discard-80", 80, 24, []string{"l", "c", "text:Half a thought", "esc"}},
		{"too-small", 64, 18, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := myproject(t, tt.w, tt.h)
			press(m, tt.keys...)
			golden(t, tt.name, screen(m))
		})
	}
}

func TestEmptyScreens(t *testing.T) {
	ctx := context.Background()

	s := newStore(t)
	golden(t, "no-projects-80", screen(open(t, s, Options{Folder: "~/code/myproject", CanSetUp: true}, 80, 24)))
	golden(t, "no-projects-home-80", screen(open(t, s, Options{Folder: "~"}, 80, 24)))

	if _, err := s.CreateProject(ctx, core.Project{Name: "myproject", TaskPrefix: "PROJ"}); err != nil {
		t.Fatal(err)
	}
	golden(t, "no-tasks-80", screen(open(t, s, Options{Project: "myproject", Folder: "~/code/myproject"}, 80, 24)))

	if _, err := s.CreateProject(ctx, core.Project{Name: "habits", TaskPrefix: "HAB"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(ctx, "habits", core.Task{Title: "Track water", Status: core.StatusTodo, Priority: core.PriorityNone}); err != nil {
		t.Fatal(err)
	}
	golden(t, "picker-80", screen(open(t, s, Options{Folder: "~/code/website", CanSetUp: true}, 80, 24)))
}

func TestMoveCard(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l") // todo: PROJ-5 high is first
	press(m, "L")
	if got := task(t, s, "PROJ-5").Status; got != core.StatusInProgress {
		t.Fatalf("after L, status = %s", got)
	}
	if sel, _ := m.selected(); sel.Ref != "PROJ-5" || m.col != 2 {
		t.Errorf("selection = %s in column %d, want PROJ-5 in 2", sel.Ref, m.col)
	}
	press(m, "H", "H")
	if got := task(t, s, "PROJ-5").Status; got != core.StatusBacklog {
		t.Errorf("after H H, status = %s", got)
	}
}

func TestQuickChanges(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l", "s", "d", "enter")
	if got := task(t, s, "PROJ-5").Status; got != core.StatusDone {
		t.Errorf("status menu: %s", got)
	}
	press(m, "p", "u", "enter")
	if got := task(t, s, "PROJ-5").Priority; got != core.PriorityUrgent {
		t.Errorf("priority menu: %s", got)
	}
	press(m, "#", "ctrl+u", "text:#Web, cli ops", "enter")
	if got := task(t, s, "PROJ-5").Labels; strings.Join(got, ",") != "cli,ops,web" {
		t.Errorf("labels: %v", got)
	}
	press(m, "#", "ctrl+u", "enter")
	if got := task(t, s, "PROJ-5").Labels; len(got) != 0 {
		t.Errorf("cleared labels: %v", got)
	}
	press(m, "#", "text:"+strings.Repeat("x", 51), "enter")
	if !strings.Contains(screen(m), "✗") {
		t.Errorf("invalid labels show no error:\n%s", screen(m))
	}
}

func TestArchiveAndRestore(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l", "j", "j", "a", "n")
	if task(t, s, "PROJ-7").ArchivedAt != nil {
		t.Fatal("n archived the task")
	}
	press(m, "a", "y")
	if task(t, s, "PROJ-7").ArchivedAt == nil {
		t.Fatal("y didn't archive the task")
	}
	press(m, "A", "a", "y")
	if task(t, s, "PROJ-7").ArchivedAt != nil {
		t.Error("restore didn't restore the task")
	}
}

func TestQuickAdd(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l", "l", "n", "text:Write the docs", "enter")
	got := task(t, s, "PROJ-10")
	if got.Title != "Write the docs" || got.Status != core.StatusInProgress {
		t.Errorf("added %+v", got)
	}
	if sel, _ := m.selected(); sel.Ref != "PROJ-10" {
		t.Errorf("selected %s after adding", sel.Ref)
	}
	press(m, "n", "esc")
	if _, err := s.GetTask(context.Background(), "PROJ-11"); err == nil {
		t.Error("esc added a task")
	}
}

func TestComments(t *testing.T) {
	m, s := myproject(t, 80, 24)
	ctx := context.Background()
	press(m, "l", "l", "j", "enter", "c", "text:Unblocked", "enter", "text:today.", "ctrl+s")
	cs, _ := s.ListComments(ctx, task(t, s, "PROJ-9").ID)
	if len(cs) != 3 || cs[2].Body != "Unblocked\ntoday." {
		t.Fatalf("comments after add: %+v", cs)
	}
	press(m, "[", "e", "ctrl+u", "text:Done.", "ctrl+s")
	cs, _ = s.ListComments(ctx, task(t, s, "PROJ-9").ID)
	if cs[2].Body != "Unblocked\nDone." {
		t.Errorf("edited comment: %q", cs[2].Body)
	}
	press(m, "]", "d", "y")
	cs, _ = s.ListComments(ctx, task(t, s, "PROJ-9").ID)
	if len(cs) != 2 {
		t.Errorf("comments after delete: %+v", cs)
	}
	press(m, "c", "ctrl+s")
	if !strings.Contains(screen(m), "The comment is empty") {
		t.Errorf("empty comment saved without a message:\n%s", screen(m))
	}
}

func TestFilter(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	for _, tt := range []struct {
		text string
		want int
	}{
		{"habit", 1},
		{"proj-9", 1},
		{"#bug", 2},
		{"status:in-progress", 2},
		{"prio:low #cli", 1},
		{"status:todo prio:high", 1},
	} {
		f, err := parseFilter(tt.text)
		if err != nil {
			t.Fatalf("%q: %v", tt.text, err)
		}
		m.filter = f
		if got, _ := m.counts(); got != tt.want {
			t.Errorf("%q matches %d tasks, want %d", tt.text, got, tt.want)
		}
	}
	if _, err := parseFilter("status:nope"); err == nil {
		t.Error("an unknown status parsed")
	}
}

func TestListSort(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	press(m, "2")
	refs := func() string {
		var r []string
		for _, t := range m.listTasks() {
			r = append(r, t.Ref)
		}
		return strings.Join(r, " ")
	}
	if got := refs(); got != "PROJ-8 PROJ-1 PROJ-5 PROJ-6 PROJ-9 PROJ-2 PROJ-3 PROJ-7 PROJ-4" {
		t.Errorf("by priority: %s", got)
	}
	press(m, "o")
	if got := refs(); got != "PROJ-3 PROJ-4 PROJ-5 PROJ-6 PROJ-7 PROJ-8 PROJ-9 PROJ-1 PROJ-2" {
		t.Errorf("by status: %s", got)
	}
	press(m, "o")
	if got := refs(); got != "PROJ-1 PROJ-2 PROJ-3 PROJ-4 PROJ-5 PROJ-6 PROJ-7 PROJ-8 PROJ-9" {
		t.Errorf("by ref: %s", got)
	}
}

func TestLaunchWithoutFolder(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	press(m, "enter", "r")
	if m.outcome.Launch != nil {
		t.Fatal("launched without a folder")
	}
	if got := screen(m); !strings.Contains(got, "✗ myproject has no folder on this machine.") ||
		!strings.Contains(got, "Run taskship init in its repository, then press r again.") {
		t.Errorf("launch error:\n%s", got)
	}
}

func TestMouse(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	screen(m)
	click := func(x, y int) {
		m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		screen(m) // records the zones of the new screen
	}
	// The todo column's first card starts below its title, in the second
	// column.
	x := 1 + (78-3)/4 + 1 + 3
	click(x, 5)
	if sel, _ := m.selected(); sel.Ref != "PROJ-5" || !m.detailOpen || m.detailRef != "PROJ-5" {
		t.Fatalf("click selected %s and opened %q (open %v), want PROJ-5 open", sel.Ref, m.detailRef, m.detailOpen)
	}
	click(m.w-5, 3) // "esc back"
	if m.detailOpen {
		t.Error("esc back didn't close the detail view")
	}
	click(13, 2) // "2 List" tab
	if m.view != viewList {
		t.Error("tab click didn't switch to the list")
	}
}

func TestSetUpFromPicker(t *testing.T) {
	s := seeded(t)
	m := open(t, s, Options{Folder: "~/code/website", CanSetUp: true}, 80, 24)
	if m.mode != modeProjects {
		t.Fatal("no project: the picker isn't open")
	}
	press(m, "j", "enter")
	if m.project.Name != "myproject" {
		t.Fatalf("enter opened %q", m.project.Name)
	}
	press(m, "P", "i")
	if !m.outcome.SetUp {
		t.Error("i didn't ask to set up the folder")
	}
}

func TestEditTaskInline(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l", "e", "text:Retry with backoff.", "tab", "ctrl+u", "text:Rate-limit retries", "enter", "ctrl+s")
	got := task(t, s, "PROJ-5")
	if got.Title != "Rate-limit retries" || got.Description != "Retry with backoff." || m.mode != modeNormal {
		t.Fatalf("after save: %q / %q, mode %d", got.Title, got.Description, m.mode)
	}

	// esc with changes asks; n keeps editing, y discards.
	press(m, "e", "text:More. ", "esc")
	if m.mode != modeConfirm {
		t.Fatalf("esc with changes didn't ask: mode %d", m.mode)
	}
	press(m, "n")
	if m.mode != modeEditTask || m.area.Value() != "More. Retry with backoff." {
		t.Fatalf("n: mode %d, text %q", m.mode, m.area.Value())
	}
	press(m, "esc", "y")
	if m.mode != modeNormal || task(t, s, "PROJ-5").Description != "Retry with backoff." {
		t.Errorf("y didn't discard: mode %d", m.mode)
	}

	// esc without changes just closes; an empty title is refused in place.
	press(m, "e", "esc")
	if m.mode != modeNormal {
		t.Errorf("esc without changes: mode %d", m.mode)
	}
	press(m, "e", "tab", "ctrl+u", "ctrl+s")
	if m.mode != modeEditTask || !strings.Contains(screen(m), "PROJ-5 wasn't saved: title must not be empty") {
		t.Errorf("empty title: mode %d\n%s", m.mode, screen(m))
	}
}

// Clicking a field's value in the detail pane works like its key.
func TestClickFieldValue(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l", "enter")
	screen(m)
	a := m.anchors["field:priority"]
	m.Update(tea.MouseClickMsg{X: a.x, Y: a.y, Button: tea.MouseLeft})
	if m.mode != modeMenu || m.menu.field != "priority" {
		t.Fatalf("click on priority: mode %d, menu %q", m.mode, m.menu.field)
	}
	press(m, "u", "enter")
	if got := task(t, s, "PROJ-5").Priority; got != core.PriorityUrgent {
		t.Errorf("priority = %s", got)
	}
	press(m, "#", "ctrl+u", "text:ops", "enter")
	if got := task(t, s, "PROJ-5").Labels; strings.Join(got, ",") != "ops" {
		t.Errorf("labels in the pane: %v", got)
	}
}

// A menu opened from the detail pane leaves the pane where it was.
func TestDetailMenuKeepsScroll(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	press(m, "l", "l", "j", "enter", "G")
	screen(m)
	scroll := m.detailScroll
	press(m, "p")
	screen(m)
	if scroll == 0 || m.detailScroll != scroll || m.mode != modeMenu {
		t.Errorf("scroll %d → %d, mode %d", scroll, m.detailScroll, m.mode)
	}
}

// e and c work as boxes over the board and leave it as it was.
func TestBoxesFromBoard(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l", "e", "text:Backoff.", "ctrl+s")
	if m.detailOpen || task(t, s, "PROJ-5").Description != "Backoff." {
		t.Fatalf("e from the board: detail open %v, description %q", m.detailOpen, task(t, s, "PROJ-5").Description)
	}
	press(m, "c", "text:Looks fine.", "ctrl+s")
	cs, _ := s.ListComments(context.Background(), task(t, s, "PROJ-5").ID)
	if m.detailOpen || len(cs) != 1 || cs[0].Body != "Looks fine." {
		t.Errorf("c from the board: detail open %v, comments %+v", m.detailOpen, cs)
	}
	if !strings.Contains(screen(m), "Added comment #1 to PROJ-5") {
		t.Errorf("no confirmation:\n%s", screen(m))
	}
}

// The comment box shows the newest earlier comments that fit and counts
// the rest.
func TestRecentComments(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	for i := 1; i <= 4; i++ {
		m.earlier = append(m.earlier, core.Comment{Number: i, Body: fmt.Sprintf("Body %d.", i)})
	}
	got := ansi.Strip(strings.Join(m.recentComments(60, 6), "\n"))
	want := "↑ 2 earlier comments\n#3 · Nov 11, 09:20\nBody 3.\n\n#4 · Nov 11, 09:20\nBody 4."
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	m.earlier = []core.Comment{{Number: 1, Body: "a\n\nb\n\nc\n\nd"}}
	if got := ansi.Strip(strings.Join(m.recentComments(60, 4), "\n")); got != "#1 · Nov 11, 09:20\na\n\n…" {
		t.Errorf("too long: %q", got)
	}
}

// Earlier comments are hidden in the comment box until ctrl-r, and hidden
// again in the next box.
func TestCommentHistoryToggle(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	press(m, "l", "l", "j", "c")
	if got := screen(m); strings.Contains(got, "Started with a plain table") || !strings.Contains(got, "ctrl-r show 2 earlier comments") {
		t.Fatalf("hidden by default:\n%s", got)
	}
	press(m, "ctrl+r")
	if got := screen(m); !strings.Contains(got, "Started with a plain table") || !strings.Contains(got, "ctrl-r hide comments") {
		t.Fatalf("shown after ctrl-r:\n%s", got)
	}
	press(m, "esc", "c")
	if strings.Contains(screen(m), "Started with a plain table") {
		t.Error("the next box shows the comments")
	}
}

// esc in the comment box asks before throwing away text.
func TestCommentDiscard(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "l", "c", "esc")
	if m.mode != modeNormal {
		t.Fatalf("esc on an empty box: mode %d", m.mode)
	}
	press(m, "c", "text:Half a thought", "esc")
	if m.mode != modeConfirm || !strings.Contains(screen(m), "Discard this comment?") {
		t.Fatalf("esc with text didn't ask:\n%s", screen(m))
	}
	press(m, "n")
	if m.mode != modeComment || m.area.Value() != "Half a thought" {
		t.Fatalf("n: mode %d, text %q", m.mode, m.area.Value())
	}
	press(m, "esc", "y")
	cs, _ := s.ListComments(context.Background(), task(t, s, "PROJ-5").ID)
	if m.mode != modeNormal || len(cs) != 0 {
		t.Errorf("y: mode %d, comments %d", m.mode, len(cs))
	}
}

const tabbedDescription = "Run it:\n\n```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```"

// The text area turns tabs into spaces; untouched text keeps its tabs.
func TestEditKeepsTabs(t *testing.T) {
	m, s := myproject(t, 80, 24)
	ctx := context.Background()
	if _, err := s.UpdateTask(ctx, "PROJ-5", store.TaskPatch{Description: ptr(tabbedDescription)}); err != nil {
		t.Fatal(err)
	}
	m.reload()
	press(m, "l", "e", "esc")
	if m.mode != modeNormal {
		t.Fatalf("esc on an untouched description asked to discard: mode %d", m.mode)
	}
	press(m, "e", "tab", "text: now", "ctrl+s")
	got := task(t, s, "PROJ-5")
	if got.Title != "Rate-limit webhook retries now" || got.Description != tabbedDescription {
		t.Errorf("after a title edit: %q\n%q", got.Title, got.Description)
	}

	c, err := s.AddComment(ctx, "PROJ-5", "Try:\n\n\tgo test ./...", "")
	if err != nil {
		t.Fatal(err)
	}
	m.startComment(got, c.Number, c.Body)
	press(m, "esc")
	if m.mode != modeNormal {
		t.Errorf("esc on an untouched comment asked to discard: mode %d", m.mode)
	}
}

func ptr[T any](v T) *T { return &v }

// A failed editor keeps the draft for e or esc.
func TestEditorFailureKeepsDraft(t *testing.T) {
	m, s := myproject(t, 80, 24)
	draft := filepath.Join(t.TempDir(), "PROJ-5.md")
	if err := os.WriteFile(draft, []byte(core.TaskMarkdown(task(t, s, "PROJ-5"))), 0o644); err != nil {
		t.Fatal(err)
	}
	m.editDraft, m.editDraftFor = draft, "PROJ-5"
	m.edited(editedMsg{"PROJ-5", draft, errors.New("exit status 1")})
	if _, err := os.Stat(draft); err != nil || m.mode != modeEditError {
		t.Fatalf("draft gone (%v) or mode %d", err, m.mode)
	}
	if got := screen(m); !strings.Contains(got, "vi failed: exit status 1") || !strings.Contains(got, "Your text is kept in") {
		t.Errorf("message:\n%s", got)
	}
	press(m, "esc")
	if _, err := os.Stat(draft); !os.IsNotExist(err) {
		t.Errorf("esc kept the draft: %v", err)
	}
}

// A rejected save from the editor names the task.
func TestEditorInvalidSaveNamesTask(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	draft := filepath.Join(t.TempDir(), "PROJ-5.md")
	if err := os.WriteFile(draft, []byte("---\nref: PROJ-5\n---\n\n# \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.editDraft, m.editDraftFor = draft, "PROJ-5"
	m.edited(editedMsg{"PROJ-5", draft, nil})
	if got := screen(m); !strings.Contains(got, "✗ PROJ-5 wasn't saved:") {
		t.Errorf("message:\n%s", got)
	}
}

// The text area takes any number of lines.
func TestLongDescription(t *testing.T) {
	m, _ := myproject(t, 80, 24)
	press(m, "l", "e")
	m.area.SetValue(strings.Repeat("line\n", 120))
	press(m, "enter")
	if n := m.area.LineCount(); n != 122 {
		t.Errorf("lines after enter = %d, want 122", n)
	}
}

// With more projects than fit, the picker scrolls to keep the cursor in
// view and says how many more there are.
func TestPickerScrolls(t *testing.T) {
	s := newStore(t)
	for i := 1; i <= 20; i++ {
		if _, err := s.CreateProject(context.Background(), core.Project{Name: fmt.Sprintf("proj%02d", i), TaskPrefix: fmt.Sprintf("P%02d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	m := open(t, s, Options{Folder: "~"}, 80, 24)
	got := screen(m)
	if !strings.Contains(got, "proj01") || strings.Contains(got, "proj20") || !strings.Contains(got, "↓") {
		t.Fatalf("first page:\n%s", got)
	}
	press(m, "G")
	got = screen(m)
	if !strings.Contains(got, "▌proj20") || strings.Contains(got, "proj01 ") || !strings.Contains(got, "↑") {
		t.Fatalf("after G:\n%s", got)
	}
	if lines := strings.Split(got, "\n"); !strings.Contains(lines[len(lines)-3], "┛") && !strings.Contains(strings.Join(lines[:len(lines)-2], "\n"), "┛") {
		t.Errorf("the box runs past the body:\n%s", got)
	}
	press(m, "enter")
	if m.project.Name != "proj20" {
		t.Errorf("enter opened %q", m.project.Name)
	}
}
