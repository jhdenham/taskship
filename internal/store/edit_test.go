package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func ptr[T any](v T) *T { return &v }

func TestUpdateTask(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	created := mustCreateTask(t, s, "myproject", core.Task{Title: "old", Description: "keep", Status: core.StatusTodo, Priority: core.PriorityNone})

	updated, err := s.UpdateTask(ctx, "PROJ-1", TaskPatch{
		Title: ptr("new"), Status: ptr(core.StatusInProgress), Priority: ptr(core.PriorityUrgent),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTask(ctx, "PROJ-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []core.Task{updated, got} {
		if task.Title != "new" || task.Description != "keep" || task.Status != core.StatusInProgress ||
			task.Priority != core.PriorityUrgent || task.ID != created.ID || !task.CreatedAt.Equal(created.CreatedAt) ||
			task.UpdatedAt.Before(created.UpdatedAt) {
			t.Errorf("after update = %+v", task)
		}
	}
	if !got.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Errorf("stored updated_at %v != returned %v", got.UpdatedAt, updated.UpdatedAt)
	}

	if _, err := s.UpdateTask(ctx, "PROJ-1", TaskPatch{Title: ptr("")}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("empty title: err = %v, want ErrInvalid", err)
	}
	if _, err := s.UpdateTask(ctx, "PROJ-9", TaskPatch{Title: ptr("x")}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown task: err = %v, want ErrNotFound", err)
	}
	if got, _ := s.GetTask(ctx, "PROJ-1"); got.Title != "new" {
		t.Errorf("failed update changed the task: %+v", got)
	}
}

func TestArchiveRestoreTask(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	mustCreateTask(t, s, "myproject", newTask("a"))
	mustCreateTask(t, s, "myproject", newTask("b"))

	for range 2 { // archiving twice is a no-op
		task, err := s.SetTaskArchived(ctx, "PROJ-1", true)
		if err != nil || task.ArchivedAt == nil {
			t.Fatalf("archive: %+v, %v", task, err)
		}
	}
	assertRefs(t, s, TaskFilter{}, "PROJ-2")
	assertRefs(t, s, TaskFilter{IncludeArchived: true}, "PROJ-1", "PROJ-2")
	if task, err := s.GetTask(ctx, "PROJ-1"); err != nil || task.ArchivedAt == nil {
		t.Errorf("archived task should still be readable: %+v, %v", task, err)
	}
	// Numbers are not reused after archiving.
	if task := mustCreateTask(t, s, "myproject", newTask("c")); task.Ref != "PROJ-3" {
		t.Errorf("new task after archive = %s, want PROJ-3", task.Ref)
	}

	for range 2 {
		task, err := s.SetTaskArchived(ctx, "PROJ-1", false)
		if err != nil || task.ArchivedAt != nil {
			t.Fatalf("restore: %+v, %v", task, err)
		}
	}
	assertRefs(t, s, TaskFilter{}, "PROJ-1", "PROJ-2", "PROJ-3")

	if _, err := s.SetTaskArchived(ctx, "PROJ-9", true); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown task: err = %v, want ErrNotFound", err)
	}
}

func TestUpdateProject(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	before, _ := s.GetProject(ctx, "myproject")
	mustCreateTask(t, s, "myproject", newTask("a"))

	p, err := s.UpdateProject(ctx, "myproject", ProjectPatch{Slug: ptr("renamed"), Name: ptr("Renamed")})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != before.ID || p.Slug != "renamed" || p.Name != "Renamed" || p.TaskPrefix != "PROJ" {
		t.Errorf("updated project = %+v", p)
	}
	if _, err := s.GetProject(ctx, "myproject"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("old slug still resolves: %v", err)
	}
	// Tasks follow the project; refs are unchanged.
	if task, err := s.GetTask(ctx, "PROJ-1"); err != nil || task.Project != "renamed" {
		t.Errorf("task after rename = %+v, %v", task, err)
	}
	// Keeping your own slug is fine; taking another project's is not.
	if _, err := s.UpdateProject(ctx, "renamed", ProjectPatch{Slug: ptr("renamed")}); err != nil {
		t.Errorf("same slug: %v", err)
	}
	if _, err := s.UpdateProject(ctx, "renamed", ProjectPatch{Slug: ptr("other")}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("slug taken: err = %v, want ErrInvalid", err)
	}
	if _, err := s.UpdateProject(ctx, "renamed", ProjectPatch{Slug: ptr("Bad Slug")}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("invalid slug: err = %v, want ErrInvalid", err)
	}
	if _, err := s.UpdateProject(ctx, "nope", ProjectPatch{Name: ptr("x")}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown project: err = %v, want ErrNotFound", err)
	}
}

func TestArchiveRestoreProject(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	mustCreateTask(t, s, "myproject", newTask("a"))
	mustCreateTask(t, s, "other", newTask("b"))

	for range 2 {
		if p, err := s.SetProjectArchived(ctx, "myproject", true); err != nil || p.ArchivedAt == nil {
			t.Fatalf("archive: %+v, %v", p, err)
		}
	}
	projects, _ := s.ListProjects(ctx, false)
	if len(projects) != 1 || projects[0].Slug != "other" {
		t.Errorf("unarchived projects = %v", projects)
	}
	if all, _ := s.ListProjects(ctx, true); len(all) != 2 {
		t.Errorf("all projects = %d, want 2", len(all))
	}
	assertRefs(t, s, TaskFilter{}, "OTH-1")
	assertRefs(t, s, TaskFilter{IncludeArchived: true}, "PROJ-1", "OTH-1")
	if _, err := s.CreateTask(ctx, "myproject", newTask("c")); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("add to archived project: err = %v, want ErrInvalid", err)
	}

	if p, err := s.SetProjectArchived(ctx, "myproject", false); err != nil || p.ArchivedAt != nil {
		t.Fatalf("restore: %+v, %v", p, err)
	}
	assertRefs(t, s, TaskFilter{}, "PROJ-1", "OTH-1")
	if _, err := s.SetProjectArchived(ctx, "nope", true); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown project: err = %v, want ErrNotFound", err)
	}
}

func assertRefs(t *testing.T, s *Store, f TaskFilter, want ...string) {
	t.Helper()
	tasks, err := s.ListTasks(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, task := range tasks {
		got = append(got, task.Ref)
	}
	if len(got) != len(want) {
		t.Fatalf("ListTasks(%+v) = %v, want %v", f, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("ListTasks(%+v) = %v, want %v", f, got, want)
		}
	}
}
