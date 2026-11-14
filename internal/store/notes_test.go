package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestNotes(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	p, _ := s.GetProject(ctx, "myproject")

	add := func(project, body string) core.Note {
		t.Helper()
		n, err := s.AddNote(ctx, project, body, "")
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	first := add("myproject", "\nTry a compact board layout\n\n")
	if first.Number != 1 || first.Body != "Try a compact board layout" || first.ProjectID != p.ID || first.RunID != "" {
		t.Errorf("first = %+v", first)
	}
	second := add("myproject", "Rename sync settings?")
	if o := add("other", "Elsewhere"); o.Number != 1 {
		t.Errorf("numbers are per project: %+v", o)
	}
	if _, err := s.AddNote(ctx, "myproject", " \n", ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("empty note: err = %v", err)
	}
	if _, err := s.AddNote(ctx, "nope", "x", ""); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing project: err = %v", err)
	}

	edited, err := s.UpdateNote(ctx, "myproject", 2, "Rename sync settings to connections?")
	if err != nil || edited.Body != "Rename sync settings to connections?" || edited.UpdatedAt.Equal(edited.CreatedAt) || !edited.CreatedAt.Equal(second.CreatedAt) {
		t.Errorf("UpdateNote = %+v, %v", edited, err)
	}
	if same, err := s.UpdateNote(ctx, "myproject", 1, first.Body); err != nil || !same.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("unchanged body counts as an edit: %+v, %v", same, err)
	}
	if _, err := s.UpdateNote(ctx, "myproject", 9, "x"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("update missing note: err = %v", err)
	}

	// Deleting the newest note doesn't free its number.
	if deleted, err := s.DeleteNote(ctx, "myproject", 2); err != nil || deleted.ID != second.ID {
		t.Errorf("DeleteNote = %+v, %v", deleted, err)
	}
	if _, err := s.DeleteNote(ctx, "myproject", 2); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("delete twice: err = %v", err)
	}
	third := add("myproject", "Third")
	if third.Number != 3 {
		t.Errorf("number after delete = %d, want 3", third.Number)
	}

	notes, err := s.ListNotes(ctx, p.ID)
	if err != nil || len(notes) != 2 || notes[0] != third || notes[1] != first {
		t.Errorf("ListNotes = %+v, %v (want newest first)", notes, err)
	}
	if got, err := s.GetNote(ctx, "myproject", 1); err != nil || got != first {
		t.Errorf("GetNote = %+v, %v", got, err)
	}

	if _, err := s.SetProjectArchived(ctx, "other", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNote(ctx, "other", "x", ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("archived project: err = %v", err)
	}
}

func TestTaskFromNote(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	p, _ := s.GetProject(ctx, "myproject")
	for _, body := range []string{"Stay", "Move me"} {
		if _, err := s.AddNote(ctx, "myproject", body, ""); err != nil {
			t.Fatal(err)
		}
	}
	backlog := core.Task{Title: "Moved", Description: "Move me", Status: core.StatusBacklog, Priority: core.PriorityNone}

	// An invalid task leaves the note alone.
	if _, err := s.TaskFromNote(ctx, "myproject", 2, core.Task{Status: core.StatusBacklog, Priority: core.PriorityNone}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("empty title: err = %v", err)
	}
	if _, err := s.TaskFromNote(ctx, "myproject", 9, backlog); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing note: err = %v", err)
	}
	task, err := s.TaskFromNote(ctx, "myproject", 2, backlog)
	if err != nil || task.Ref != "PROJ-1" || task.Status != core.StatusBacklog || task.Description != "Move me" {
		t.Fatalf("TaskFromNote = %+v, %v", task, err)
	}
	if _, err := s.GetNote(ctx, "myproject", 2); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("note kept after making a task: err = %v", err)
	}
	if notes, _ := s.ListNotes(ctx, p.ID); len(notes) != 1 || notes[0].Body != "Stay" {
		t.Errorf("other notes changed: %+v", notes)
	}
}

func TestNoteRuns(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	task := mustCreateTask(t, s, "myproject", newTask("Fix export"))
	foreign := mustCreateTask(t, s, "other", newTask("Elsewhere"))
	newRun := func(task core.Task, start bool) string {
		t.Helper()
		r, err := s.CreateRun(ctx, core.Run{TaskID: task.ID, TaskRef: task.Ref, Agent: "claude", Machine: "m", WorkspacePath: "/w"})
		if err != nil {
			t.Fatal(err)
		}
		if start {
			if err := s.StartRun(ctx, r.ID); err != nil {
				t.Fatal(err)
			}
		}
		return r.ID
	}
	running := newRun(task, true)
	starting := newRun(task, false)
	otherProject := newRun(foreign, true)

	// Notes record a running run of a task in their project, and nothing else.
	for id, want := range map[string]string{running: running, starting: "", otherProject: "", "nope": "", "": ""} {
		n, err := s.AddNote(ctx, "myproject", "note", id)
		if err != nil || n.RunID != want {
			t.Errorf("note with run %q: RunID %q, %v", id, n.RunID, err)
		}
		if want != "" {
			if got, _ := s.GetNote(ctx, "myproject", n.Number); got.Run == nil || got.Run.String() != "Claude (run #1)" {
				t.Errorf("stored note run = %v", got.Run)
			}
		}
	}
}
