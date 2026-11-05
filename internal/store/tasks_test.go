package store

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func newTask(title string) core.Task {
	return core.Task{Title: title, Status: core.StatusTodo, Priority: core.PriorityNone}
}

func seedProjects(t *testing.T, s *Store) {
	t.Helper()
	for _, p := range []core.Project{
		{Slug: "myproject", Name: "myproject", TaskPrefix: "PROJ"},
		{Slug: "other", Name: "other", TaskPrefix: "OTH"},
	} {
		if _, err := s.CreateProject(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
}

func mustCreateTask(t *testing.T, s *Store, project string, task core.Task) core.Task {
	t.Helper()
	created, err := s.CreateTask(context.Background(), project, task)
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func TestCreateTaskNumbersPerProject(t *testing.T) {
	s := openTest(t, t.TempDir())
	seedProjects(t, s)

	var refs []string
	for _, c := range []struct{ project, title string }{
		{"myproject", "a"}, {"myproject", "b"}, {"other", "c"}, {"myproject", "d"},
	} {
		refs = append(refs, mustCreateTask(t, s, c.project, newTask(c.title)).Ref)
	}
	want := []string{"PROJ-1", "PROJ-2", "OTH-1", "PROJ-3"}
	for i := range want {
		if refs[i] != want[i] {
			t.Errorf("refs = %v, want %v", refs, want)
			break
		}
	}
}

func TestTaskRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)

	in := core.Task{Title: "Fix bug", Description: "details", Status: core.StatusBacklog, Priority: core.PriorityHigh}
	created := mustCreateTask(t, s, "myproject", in)
	if created.ID == "" || created.Project != "myproject" || created.ProjectID == "" || created.CreatedAt.IsZero() {
		t.Errorf("created task incomplete: %+v", created)
	}

	got, err := s.GetTask(ctx, "proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Ref != "PROJ-1" || got.Project != "myproject" || got.ProjectID != created.ProjectID ||
		got.Title != in.Title || got.Description != in.Description || got.Status != in.Status ||
		got.Priority != in.Priority || !got.CreatedAt.Equal(created.CreatedAt) || got.ArchivedAt != nil {
		t.Errorf("GetTask = %+v\nwant %+v", got, created)
	}
}

func TestCreateTaskErrors(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)

	if _, err := s.CreateTask(ctx, "nope", newTask("x")); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown project: err = %v, want ErrNotFound", err)
	}
	bad := newTask("x")
	bad.Status = "open"
	if _, err := s.CreateTask(ctx, "myproject", bad); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("invalid status: err = %v, want ErrInvalid", err)
	}
}

func TestGetTaskErrors(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	mustCreateTask(t, s, "myproject", newTask("a"))

	for ref, want := range map[string]error{
		"PROJ-2":    core.ErrNotFound,
		"NOPE-1":    core.ErrNotFound,
		"not-a-ref": core.ErrInvalid,
	} {
		if _, err := s.GetTask(ctx, ref); !errors.Is(err, want) {
			t.Errorf("GetTask(%q) err = %v, want %v", ref, err, want)
		}
	}
}

func TestListTasksFilters(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)

	for _, c := range []struct {
		project string
		status  core.Status
	}{
		{"other", core.StatusTodo},           // OTH-1
		{"myproject", core.StatusTodo},       // PROJ-1
		{"myproject", core.StatusDone},       // PROJ-2
		{"myproject", core.StatusInProgress}, // PROJ-3
	} {
		task := newTask("t")
		task.Status = c.status
		mustCreateTask(t, s, c.project, task)
	}

	tests := []struct {
		name   string
		filter TaskFilter
		want   []string
	}{
		{"all, ordered by project then number", TaskFilter{}, []string{"PROJ-1", "PROJ-2", "PROJ-3", "OTH-1"}},
		{"by project", TaskFilter{ProjectSlug: "other"}, []string{"OTH-1"}},
		{"by status", TaskFilter{Statuses: []core.Status{core.StatusTodo}}, []string{"PROJ-1", "OTH-1"}},
		{"by project and statuses", TaskFilter{ProjectSlug: "myproject", Statuses: []core.Status{core.StatusDone, core.StatusInProgress}}, []string{"PROJ-2", "PROJ-3"}},
		{"no matches", TaskFilter{Statuses: []core.Status{core.StatusCanceled}}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := s.ListTasks(ctx, tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, task := range tasks {
				got = append(got, task.Ref)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("refs = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("refs = %v, want %v", got, tt.want)
				}
			}
		})
	}

	if _, err := s.ListTasks(ctx, TaskFilter{ProjectSlug: "nope"}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown project filter: err = %v, want ErrNotFound", err)
	}
}

// Separate Store instances simulate separate taskship processes.
func TestCreateTaskConcurrentNumbering(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s1 := openTest(t, dir)
	seedProjects(t, s1)
	s2 := openTest(t, dir)

	const perStore = 15
	var mu sync.Mutex
	var seqs []int
	var wg sync.WaitGroup
	for _, s := range []*Store{s1, s2} {
		for range perStore {
			wg.Add(1)
			go func() {
				defer wg.Done()
				task, err := s.CreateTask(ctx, "myproject", newTask("t"))
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				seqs = append(seqs, task.Sequence)
				mu.Unlock()
			}()
		}
	}
	wg.Wait()

	sort.Ints(seqs)
	if len(seqs) != 2*perStore {
		t.Fatalf("created %d tasks, want %d", len(seqs), 2*perStore)
	}
	for i, seq := range seqs {
		if seq != i+1 {
			t.Fatalf("sequence numbers not unique and contiguous: %v", seqs)
		}
	}
}
