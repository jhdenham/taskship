package dispatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
	"github.com/jhdenham/taskship/internal/workspace"
)

func newTask(t *testing.T) (*store.Store, core.Task) {
	t.Helper()
	ctx := context.Background()
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.CreateProject(ctx, core.Project{Name: "myproject", TaskPrefix: "PROJ"}); err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(ctx, "myproject", core.Task{Title: "Fix it", Status: core.StatusTodo, Priority: core.PriorityNone})
	if err != nil {
		t.Fatal(err)
	}
	return s, task
}

func TestPrepareWithoutFolder(t *testing.T) {
	s, task := newTask(t)
	_, err := Prepare(context.Background(), s, task, core.RunModeRun, t.TempDir(), false)
	var nf *workspace.NoFolderError
	if !errors.As(err, &nf) || nf.Project != "myproject" {
		t.Fatalf("Prepare error = %v, want a NoFolderError", err)
	}
}

func TestReadyWithoutAgent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := Plan{Prompt: "hi"}
	if err := p.Ready(); !errors.Is(err, ErrAgentMissing) {
		t.Fatalf("Ready error = %v, want ErrAgentMissing", err)
	}
}

// An agent that can't be started leaves the task's status as it was and its
// run recorded as failed.
func TestExecuteStartFailureRestoresStatus(t *testing.T) {
	ctx := context.Background()
	s, task := newTask(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/nonexistent/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	p := Plan{Task: task, Agent: "claude", Dir: t.TempDir(), Prompt: "hi"}
	if err := p.Ready(); err != nil {
		t.Fatal(err)
	}
	var started bool
	_, err := Execute(ctx, s, p, func(bool) { started = true }, func(err error) { t.Errorf("warning: %v", err) })
	var se *StartError
	if !errors.As(err, &se) || !started {
		t.Fatalf("Execute error = %v (started %v), want a StartError after starting", err, started)
	}
	got, err := s.GetTask(ctx, task.Ref)
	if err != nil || got.Status != core.StatusTodo {
		t.Errorf("status after failed start = %q (%v), want todo", got.Status, err)
	}
	runs, err := s.ListRuns(ctx, task.ID)
	if err != nil || len(runs) != 1 || runs[0].State != core.RunFailed {
		t.Errorf("runs = %+v (%v), want one failed run", runs, err)
	}
}
