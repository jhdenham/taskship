package store

import (
	"context"
	"errors"
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
	if ended.ID == "" || ended.State != core.RunStarting || ended.StartedAt.IsZero() {
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
