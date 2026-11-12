package store

import (
	"context"
	"errors"
	"reflect"
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
		{Name: "myproject", TaskPrefix: "PROJ"},
		{Name: "other", TaskPrefix: "OTH"},
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
		{"by project", TaskFilter{Project: "other"}, []string{"OTH-1"}},
		{"by status", TaskFilter{Statuses: []core.Status{core.StatusTodo}}, []string{"PROJ-1", "OTH-1"}},
		{"by project and statuses", TaskFilter{Project: "myproject", Statuses: []core.Status{core.StatusDone, core.StatusInProgress}}, []string{"PROJ-2", "PROJ-3"}},
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

	if _, err := s.ListTasks(ctx, TaskFilter{Project: "nope"}); !errors.Is(err, core.ErrNotFound) {
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

func TestTaskLabels(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	in := newTask("Labeled")
	in.Labels = []string{"Infra", "cli", "infra"}
	a := mustCreateTask(t, s, "myproject", in)
	if !reflect.DeepEqual(a.Labels, []string{"cli", "infra"}) {
		t.Errorf("created labels = %q", a.Labels)
	}
	b := mustCreateTask(t, s, "myproject", newTask("Plain"))
	if got, _ := s.GetTask(ctx, b.Ref); got.Labels == nil || len(got.Labels) != 0 {
		t.Errorf("no labels = %#v, want empty, not nil", got.Labels)
	}
	if got, _ := s.GetTask(ctx, a.Ref); !reflect.DeepEqual(got.Labels, []string{"cli", "infra"}) {
		t.Errorf("stored labels = %q", got.Labels)
	}

	bad := newTask("Bad")
	bad.Labels = []string{"two words"}
	if _, err := s.CreateTask(ctx, "myproject", bad); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("invalid label: err = %v", err)
	}

	// A patch replaces the set; no Labels in the patch leaves it alone.
	set := []string{"bug", "CLI"}
	if got, err := s.UpdateTask(ctx, a.Ref, TaskPatch{Labels: &set}); err != nil || !reflect.DeepEqual(got.Labels, []string{"bug", "cli"}) {
		t.Errorf("UpdateTask labels = %q, %v", got.Labels, err)
	}
	title := "Renamed"
	if got, err := s.UpdateTask(ctx, a.Ref, TaskPatch{Title: &title}); err != nil || !reflect.DeepEqual(got.Labels, []string{"bug", "cli"}) {
		t.Errorf("title-only patch labels = %q, %v", got.Labels, err)
	}
	if got, _ := s.GetTask(ctx, a.Ref); !reflect.DeepEqual(got.Labels, []string{"bug", "cli"}) {
		t.Errorf("stored after patch = %q", got.Labels)
	}

	refs := func(f TaskFilter) []string {
		t.Helper()
		tasks, err := s.ListTasks(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		var r []string
		for _, task := range tasks {
			r = append(r, task.Ref)
		}
		return r
	}
	if got := refs(TaskFilter{Labels: []string{"BUG"}}); !reflect.DeepEqual(got, []string{a.Ref}) {
		t.Errorf("label filter = %q", got)
	}
	if got := refs(TaskFilter{Labels: []string{"bug", "infra"}}); got != nil {
		t.Errorf("all-labels filter = %q, want none", got)
	}

	empty := []string{}
	if got, err := s.UpdateTask(ctx, a.Ref, TaskPatch{Labels: &empty}); err != nil || len(got.Labels) != 0 {
		t.Errorf("clear labels = %q, %v", got.Labels, err)
	}
}

func TestCopyTask(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	in := core.Task{Title: "Fix export", Description: "## Acceptance criteria\n\n- [ ] CSV", Status: core.StatusDone,
		Priority: core.PriorityHigh, Labels: []string{"bug", "cli"}}
	src := mustCreateTask(t, s, "myproject", in)
	if _, err := s.AddComment(ctx, src.Ref, "History", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTaskArchived(ctx, src.Ref, true); err != nil {
		t.Fatal(err)
	}

	same, original, err := s.CopyTask(ctx, "proj-1", "", "", core.StatusTodo)
	if err != nil {
		t.Fatal(err)
	}
	if original.Ref != src.Ref || original.ID != src.ID {
		t.Errorf("original = %+v", original)
	}
	if same.Ref != "PROJ-2" || same.Title != in.Title || same.Description != in.Description || same.Priority != in.Priority ||
		!reflect.DeepEqual(same.Labels, in.Labels) || same.Status != core.StatusTodo || same.ArchivedAt != nil || same.CompletedAt != nil {
		t.Errorf("copy = %+v", same)
	}
	comments, _ := s.ListComments(ctx, same.ID)
	if len(comments) != 1 || comments[0].Body != "Copied from PROJ-1." || comments[0].Number != 1 {
		t.Errorf("copy comments = %+v", comments)
	}

	other, _, err := s.CopyTask(ctx, "proj-1", "other", "Fix export in other", core.StatusBacklog)
	if err != nil || other.Ref != "OTH-1" || other.Project != "other" || other.Title != "Fix export in other" || other.Status != core.StatusBacklog {
		t.Errorf("copy to other = %+v, %v", other, err)
	}

	for _, c := range []struct {
		ref, project string
		want         error
	}{{"PROJ-9", "", core.ErrNotFound}, {"PROJ-1", "nope", core.ErrNotFound}} {
		if _, _, err := s.CopyTask(ctx, c.ref, c.project, "", core.StatusTodo); !errors.Is(err, c.want) {
			t.Errorf("CopyTask(%s, %q): err = %v, want %v", c.ref, c.project, err, c.want)
		}
	}
	// A failed copy leaves nothing behind.
	if _, _, err := s.CopyTask(ctx, "PROJ-1", "", "two\nlines", core.StatusTodo); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("invalid title: err = %v", err)
	}
	if tasks, _ := s.ListTasks(ctx, TaskFilter{IncludeArchived: true}); len(tasks) != 3 {
		t.Errorf("tasks after copies = %d, want 3", len(tasks))
	}
}

func TestCreateDoneTaskIsCompleted(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	in := newTask("Already done")
	in.Status = core.StatusDone
	created := mustCreateTask(t, s, "myproject", in)
	if created.CompletedAt == nil || !created.CompletedAt.Equal(created.CreatedAt) {
		t.Errorf("created done = %+v", created)
	}
	if got, _ := s.GetTask(ctx, created.Ref); !reflect.DeepEqual(got, created) {
		t.Errorf("stored %+v, created %+v", got, created)
	}
	if todo := mustCreateTask(t, s, "myproject", newTask("Todo")); todo.CompletedAt != nil {
		t.Errorf("todo task completed: %+v", todo)
	}
	copied, _, err := s.CopyTask(ctx, created.Ref, "", "", core.StatusDone)
	if err != nil || copied.CompletedAt == nil {
		t.Errorf("copied as done = %+v, %v", copied, err)
	}
}
