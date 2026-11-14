package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

func init() {
	timeAgo = func(time.Time) string { return "Nov 11" }
}

// sampleNotes are the design brief's sample notes, oldest first; the
// retry-loop note comes from a run of Claude on PROJ-5.
var sampleNotes = []string{
	`Rename "sync settings" to "connections"?`,
	"From the PROJ-23 session:\n\n> The login test is flaky because it depends on the system clock.\n> I'd inject a clock into the session store rather than sleeping\n> in the test. That touches auth/session.go and three tests.",
	"Handy query for finding stale sessions:\n\n```sql\nSELECT id, user_id, last_seen\nFROM sessions\nWHERE last_seen < now() - interval '30 days'\nORDER BY last_seen;\n```\n\nCould become a cleanup job.",
	"Ask about rate limits before we publish the webhook docs",
	"The retry loop in sync/client.go never resets its backoff after a success. Probably why PROJ-18 keeps coming back.",
	"Try a compact board layout",
}

// withNotes adds the sample notes to myproject.
func withNotes(t *testing.T, s *store.Store) {
	t.Helper()
	ctx := context.Background()
	p5 := task(t, s, "PROJ-5")
	r, err := s.CreateRun(ctx, core.Run{TaskID: p5.ID, TaskRef: p5.Ref, Agent: "claude", Machine: "m", WorkspacePath: "/w"})
	if err == nil {
		err = s.StartRun(ctx, r.ID)
	}
	if err != nil {
		t.Fatal(err)
	}
	for i, body := range sampleNotes {
		run := ""
		if i == 4 {
			run = r.ID
		}
		if _, err := s.AddNote(ctx, "myproject", body, run); err != nil {
			t.Fatal(err)
		}
	}
}

func notesProject(t *testing.T, w, h int) (*model, *store.Store) {
	s := seeded(t)
	withNotes(t, s)
	return open(t, s, Options{Project: "myproject", Folder: "~/code/myproject"}, w, h), s
}

func TestNoteScreens(t *testing.T) {
	for _, tt := range []struct {
		name string
		w, h int
		keys []string
	}{
		{"notes-80", 80, 24, []string{"3", "l"}},
		{"notes-180", 180, 44, []string{"3"}},
		{"notes-open-80", 80, 24, []string{"3", "j", "l", "enter"}},
		{"notes-open-180", 180, 44, []string{"3", "j", "enter"}},
		{"notes-new-80", 80, 24, []string{"3", "n", "text:Try a compact board lay"}},
		{"notes-edit-180", 180, 44, []string{"3", "j", "e"}},
		{"notes-delete-80", 80, 24, []string{"3", "j", "d"}},
		{"notes-task-180", 180, 44, []string{"3", "l", "t"}},
		{"notes-filter-180", 180, 44, []string{"3", "/", "text:PROJ", "enter"}},
		{"notes-filter-none-80", 80, 24, []string{"3", "/", "text:sesions", "enter"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := notesProject(t, tt.w, tt.h)
			press(m, tt.keys...)
			golden(t, tt.name, screen(m))
		})
	}
	m, _ := myproject(t, 80, 24)
	press(m, "3")
	golden(t, "notes-empty-80", screen(m))
}

func notes(t *testing.T, s *store.Store) []core.Note {
	t.Helper()
	p, _ := s.GetProject(context.Background(), "myproject")
	ns, err := s.ListNotes(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return ns
}

func TestNoteBox(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "3", "n", "text:First line", "enter", "text:second.", "ctrl+s")
	if ns := notes(t, s); len(ns) != 1 || ns[0].Body != "First line\nsecond." {
		t.Fatalf("notes after add: %+v", ns)
	}
	press(m, "e", "ctrl+u", "text:Changed.", "ctrl+s")
	if ns := notes(t, s); ns[0].Body != "First line\nChanged." {
		t.Errorf("edited note: %q", ns[0].Body)
	}
	press(m, "n", "ctrl+s")
	if !strings.Contains(screen(m), "The note is empty") {
		t.Errorf("empty note saved without a message:\n%s", screen(m))
	}
	// esc asks before throwing away text.
	press(m, "text:Half", "esc")
	if m.mode != modeConfirm {
		t.Fatalf("esc with text: mode %d", m.mode)
	}
	press(m, "y")
	if m.mode != modeNormal || len(notes(t, s)) != 1 {
		t.Errorf("discard: mode %d, notes %+v", m.mode, notes(t, s))
	}
	press(m, "d", "n")
	if len(notes(t, s)) != 1 {
		t.Errorf("n deleted the note")
	}
	press(m, "d", "y")
	if len(notes(t, s)) != 0 || !strings.Contains(screen(m), "No notes in myproject yet.") {
		t.Errorf("after delete: %+v\n%s", notes(t, s), screen(m))
	}
}

func TestTaskFromNote(t *testing.T) {
	m, s := notesProject(t, 180, 44)
	press(m, "3", "l") // the retry-loop note
	// Without a title, nothing is added and the box stays open.
	press(m, "t", "ctrl+s")
	if m.mode != modeEditTask || !strings.Contains(screen(m), "Write a title for the task, or press esc to cancel.") {
		t.Fatalf("empty title: mode %d\n%s", m.mode, screen(m))
	}
	press(m, "text:Reset the sync backoff", "ctrl+s")
	got := task(t, s, "PROJ-10")
	if got.Title != "Reset the sync backoff" || got.Status != core.StatusBacklog || got.Description != sampleNotes[4] {
		t.Errorf("task = %+v", got)
	}
	if ns := notes(t, s); len(ns) != 5 || ns[1].Body == sampleNotes[4] {
		t.Errorf("note kept: %+v", ns)
	}
	if !strings.Contains(screen(m), "✓ Added PROJ-10: Reset the sync backoff") {
		t.Errorf("no message:\n%s", screen(m))
	}
}

func TestNoteGridMoves(t *testing.T) {
	m, _ := notesProject(t, 80, 24) // two columns
	press(m, "3", "l", "j")
	if n, _ := m.selectedNote(); n.Number != 3 {
		t.Errorf("after l j: note #%d, want #3 (row 2, column 2)", n.Number)
	}
	press(m, "l", "k", "k", "h", "h")
	if n, _ := m.selectedNote(); n.Number != 6 {
		t.Errorf("at the start: note #%d", n.Number)
	}
	press(m, "G", "enter")
	if !m.noteOpen || m.noteNum != 1 {
		t.Errorf("G enter: open %v on #%d", m.noteOpen, m.noteNum)
	}
	press(m, "esc", "/", "text:query", "enter")
	if ns := m.shownNotes(); len(ns) != 1 || m.selNote != 3 {
		t.Errorf("filter: %d shown, selected #%d", len(ns), m.selNote)
	}
}

// Notes added elsewhere show up like tasks do.
func TestNotesRefresh(t *testing.T) {
	m, s := myproject(t, 80, 24)
	press(m, "3")
	if _, err := s.AddNote(context.Background(), "myproject", "From an agent", ""); err != nil {
		t.Fatal(err)
	}
	m.Update(checkMsg{})
	if !strings.Contains(screen(m), "From an agent") {
		t.Errorf("note added elsewhere doesn't show:\n%s", screen(m))
	}
}

func TestLaneScreens(t *testing.T) {
	for _, tt := range []struct {
		name string
		w, h int
		keys []string
	}{
		{"lane-80", 80, 24, []string{"N", "h", "j"}},
		{"lane-180", 180, 44, []string{"N", "h", "j"}},
		{"lane-open-180", 180, 44, []string{"N", "h", "j", "enter"}},
		{"lane-add-80", 80, 24, []string{"N", "h", "n", "text:Check the"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := notesProject(t, tt.w, tt.h)
			press(m, tt.keys...)
			golden(t, tt.name, screen(m))
		})
	}
}

func TestLane(t *testing.T) {
	m, s := notesProject(t, 80, 24)
	press(m, "h")
	if m.inLane {
		t.Fatal("h entered the lane while it was hidden")
	}
	press(m, "N", "h", "j")
	if n, _ := m.selectedNote(); !m.inLane || n.Number != 5 {
		t.Fatalf("lane selection: in lane %v, note #%d", m.inLane, n.Number)
	}
	// t makes a backlog task from the lane's note.
	press(m, "t", "text:Reset the sync backoff", "ctrl+s")
	if got := task(t, s, "PROJ-10"); got.Status != core.StatusBacklog || len(notes(t, s)) != 5 {
		t.Errorf("task from the lane: %+v", got)
	}
	// n adds a note while in the lane, and a task outside it.
	// n adds a one-line note in place, like a task in a column.
	press(m, "n", "text:From the board", "enter")
	if ns := notes(t, s); ns[0].Body != "From the board" || m.mode != modeNormal {
		t.Errorf("n in the lane: %+v", ns[0])
	}
	if n, _ := m.selectedNote(); n.Body != "From the board" {
		t.Errorf("new note not selected: %+v", n)
	}
	press(m, "n", "esc")
	if len(notes(t, s)) != 6 {
		t.Errorf("esc added a note")
	}
	press(m, "l")
	if sel, _ := m.selected(); m.inLane || sel.Ref != "PROJ-3" {
		t.Errorf("l left the lane to %s (in lane %v)", sel.Ref, m.inLane)
	}
	press(m, "h", "N")
	if m.inLane || m.showLane {
		t.Errorf("N left the lane shown (%v) or selected (%v)", m.showLane, m.inLane)
	}
}

// Notes aren't archived, so the lane hides with the archived tasks.
func TestLaneHiddenWhenArchived(t *testing.T) {
	m, _ := notesProject(t, 80, 24)
	press(m, "N", "h", "A")
	if m.inLane || strings.Contains(screen(m), "notes 6") {
		t.Errorf("lane with archived tasks (in lane %v):\n%s", m.inLane, screen(m))
	}
	press(m, "A")
	if !strings.Contains(screen(m), "notes 6") {
		t.Errorf("lane didn't come back:\n%s", screen(m))
	}
}

// With archived tasks shown, the footer offers A to go back instead of N.
func TestArchivedFooter(t *testing.T) {
	for _, w := range []int{80, 180} {
		m, _ := notesProject(t, w, 24)
		press(m, "l", "j", "j", "a", "y", "A", "1")
		lines := strings.Split(strings.TrimRight(screen(m), "\n"), "\n")
		footer := lines[len(lines)-1]
		if strings.Contains(footer, "N ") || !strings.Contains(footer, "A hide archived") || !strings.HasSuffix(footer, "q quit") {
			t.Errorf("archived board footer at %d: %q", w, footer)
		}
	}
}

// Review fixes: clicking a card leaves the lane, the lane ignores the Notes
// view's filter and shows on an empty board, and saving from the editor
// without the box says what happened.
func TestLaneEdges(t *testing.T) {
	m, _ := notesProject(t, 80, 24)
	press(m, "N", "h")
	screen(m)
	m.Update(tea.MouseClickMsg{X: 22, Y: 5, Button: tea.MouseLeft}) // backlog's first card
	if m.inLane || !m.detailOpen || m.detailRef != "PROJ-3" {
		t.Errorf("click on a card from the lane: in lane %v, detail %v on %s", m.inLane, m.detailOpen, m.detailRef)
	}

	m, _ = notesProject(t, 80, 24)
	press(m, "3", "/", "text:query", "enter", "1", "N")
	if !strings.Contains(screen(m), "notes 6") {
		t.Errorf("the Notes view's filter narrowed the lane:\n%s", screen(m))
	}

	s := newStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, core.Project{Name: "myproject", TaskPrefix: "PROJ"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNote(ctx, "myproject", "Only a note", ""); err != nil {
		t.Fatal(err)
	}
	m = open(t, s, Options{Project: "myproject", Folder: "~/code/myproject"}, 80, 24)
	press(m, "N", "h")
	if out := screen(m); !strings.Contains(out, "Only a note") || !strings.Contains(out, "t make task") {
		t.Errorf("lane on a board without tasks:\n%s", out)
	}

	// As after the lane's ctrl-g: the editor returns with no box open.
	m.editNote, m.noteOrig = 1, "Only a note"
	m.saveNote("  ")
	if out := screen(m); !strings.Contains(out, "The note is empty, so it wasn't changed.") {
		t.Errorf("empty text from the editor:\n%s", out)
	}
	m.saveNote("Only a note")
	if out := screen(m); !strings.Contains(out, "No changes to the note") {
		t.Errorf("unchanged text from the editor:\n%s", out)
	}
}
