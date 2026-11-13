package store

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestRuns(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	task := mustCreateTask(t, s, "myproject", newTask("Fix export"))
	other := mustCreateTask(t, s, "myproject", newTask("Other"))

	run := func() core.Run {
		t.Helper()
		r, err := s.CreateRun(ctx, core.Run{TaskID: task.ID, TaskRef: task.Ref, Agent: "claude",
			Machine: "laptop", WorkspacePath: "/code/myproject", SessionID: "sess"})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	ended, failed, running := run(), run(), run()
	if ended.Number != 1 || failed.Number != 2 || running.Number != 3 {
		t.Errorf("numbers = %d, %d, %d", ended.Number, failed.Number, running.Number)
	}
	if ended.ID == "" || ended.State != core.RunStarting || ended.StartedAt.IsZero() || ended.Mode != core.RunModeRun {
		t.Errorf("created = %+v", ended)
	}

	for _, id := range []string{ended.ID, running.ID} {
		if err := s.StartRun(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	code := 3
	if err := s.FinishRun(ctx, ended.ID, &code); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(ctx, failed.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.StartRun(ctx, "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("StartRun unknown: err = %v, want ErrNotFound", err)
	}

	// Refining shares the task's numbering and keeps its mode.
	refined := mustCreateTask(t, s, "myproject", newTask("Refined"))
	refine, err := s.CreateRun(ctx, core.Run{TaskID: refined.ID, TaskRef: refined.Ref, Agent: "claude", Mode: core.RunModeRefine,
		Machine: "laptop", WorkspacePath: "/code/myproject"})
	if err != nil || refine.Number != 1 {
		t.Fatalf("CreateRun refine = %+v, %v", refine, err)
	}
	if runs, err := s.ListRuns(ctx, refined.ID); err != nil || len(runs) != 1 || runs[0].Mode != core.RunModeRefine {
		t.Errorf("ListRuns refine = %+v, %v", runs, err)
	}

	runs, err := s.ListRuns(ctx, task.ID)
	if err != nil || len(runs) != 3 {
		t.Fatalf("ListRuns = %+v, %v", runs, err)
	}
	// Newest first.
	got := map[string]core.Run{}
	for _, r := range runs {
		got[r.ID] = r
	}
	if runs[0].ID != running.ID || runs[2].ID != ended.ID {
		t.Errorf("order = %s, %s, %s", runs[0].ID, runs[1].ID, runs[2].ID)
	}
	if r := got[ended.ID]; r.State != core.RunEnded || r.ExitCode == nil || *r.ExitCode != 3 || r.EndedAt == nil ||
		r.SessionID != "sess" || r.TaskRef != "PROJ-1" || r.Machine != "laptop" || r.WorkspacePath != "/code/myproject" {
		t.Errorf("ended run = %+v", r)
	}
	if r := got[failed.ID]; r.State != core.RunFailed || r.ExitCode != nil || r.EndedAt == nil {
		t.Errorf("failed run = %+v", r)
	}
	if r := got[running.ID]; r.State != core.RunRunning || r.ExitCode != nil || r.EndedAt != nil {
		t.Errorf("running run = %+v", r)
	}
	if runs, err := s.ListRuns(ctx, other.ID); err != nil || len(runs) != 0 {
		t.Errorf("other task runs = %+v, %v", runs, err)
	}
}

func TestCompletionAndRunLinks(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	task := mustCreateTask(t, s, "myproject", newTask("Fix export"))
	other := mustCreateTask(t, s, "myproject", newTask("Other"))
	newRun := func(taskID, ref string, start bool) string {
		t.Helper()
		r, err := s.CreateRun(ctx, core.Run{TaskID: taskID, TaskRef: ref, Agent: "claude", Machine: "m", WorkspacePath: "/w"})
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
	running := newRun(task.ID, task.Ref, true)
	otherRun := newRun(other.ID, other.Ref, true)
	ended := newRun(task.ID, task.Ref, true)
	code := 0
	if err := s.FinishRun(ctx, ended, &code); err != nil {
		t.Fatal(err)
	}
	status := func(st core.Status, runID string) core.Task {
		t.Helper()
		got, err := s.UpdateTask(ctx, task.Ref, TaskPatch{Status: &st, RunID: runID})
		if err != nil {
			t.Fatal(err)
		}
		if stored, _ := s.GetTask(ctx, task.Ref); !reflect.DeepEqual(stored, got) {
			t.Errorf("stored %+v, returned %+v", stored, got)
		}
		return got
	}

	// Done without a run: completed now, by no run.
	if got := status(core.StatusDone, ""); got.CompletedAt == nil || !got.CompletedAt.Equal(got.UpdatedAt) || got.CompletedRunID != "" {
		t.Errorf("done = %+v", got)
	}
	// Leaving done clears it; a stale or foreign run is never credited.
	if got := status(core.StatusTodo, ""); got.CompletedAt != nil {
		t.Errorf("reopened = %+v", got)
	}
	for _, id := range []string{ended, otherRun, "nope"} {
		if got := status(core.StatusDone, id); got.CompletedAt == nil || got.CompletedRunID != "" {
			t.Errorf("run %s credited: %+v", id, got)
		}
		status(core.StatusTodo, "")
	}
	// A running run of the task is credited, and staying done keeps the record.
	first := status(core.StatusDone, running)
	if first.CompletedRunID != running || first.CompletedBy == nil || first.CompletedBy.String() != "Claude (run #1)" {
		t.Errorf("running run not credited: %+v", first)
	}
	title := "Renamed"
	if got, err := s.UpdateTask(ctx, task.Ref, TaskPatch{Title: &title}); err != nil || !got.CompletedAt.Equal(*first.CompletedAt) || got.CompletedRunID != running {
		t.Errorf("edit while done = %+v, %v", got, err)
	}

	// Comments record a running run of their task, and nothing else.
	for id, want := range map[string]string{running: running, ended: "", otherRun: "", "": ""} {
		if c, err := s.AddComment(ctx, task.Ref, "note", id); err != nil || c.RunID != want {
			t.Errorf("comment with run %q: RunID %q, %v", id, c.RunID, err)
		}
	}
	comments, _ := s.ListComments(ctx, task.ID)
	linked := 0
	for _, c := range comments {
		if c.RunID == running {
			linked++
			if c.Run == nil || c.Run.String() != "Claude (run #1)" {
				t.Errorf("linked comment run = %v", c.Run)
			}
		} else if c.RunID != "" {
			t.Errorf("comment linked to %q", c.RunID)
		}
	}
	if len(comments) != 4 || linked != 1 {
		t.Errorf("stored comments = %+v", comments)
	}
}
