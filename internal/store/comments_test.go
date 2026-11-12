package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestComments(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	task := mustCreateTask(t, s, "myproject", newTask("Fix export"))
	other := mustCreateTask(t, s, "myproject", newTask("Other"))

	add := func(ref, body string) core.Comment {
		t.Helper()
		c, err := s.AddComment(ctx, ref, body, "")
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	first := add(task.Ref, "\nTried bumping the timeout.\n\n")
	if first.Number != 1 || first.Body != "Tried bumping the timeout." || first.TaskID != task.ID || first.Edited() {
		t.Errorf("first = %+v", first)
	}
	second := add("proj-1", "Still flaky.")
	if o := add(other.Ref, "Elsewhere"); o.Number != 1 {
		t.Errorf("numbers are per task: %+v", o)
	}
	if _, err := s.AddComment(ctx, task.Ref, "  \n", ""); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("empty comment: err = %v", err)
	}
	if _, err := s.AddComment(ctx, "PROJ-99", "x", ""); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing task: err = %v", err)
	}

	edited, err := s.UpdateComment(ctx, task.Ref, 2, "Still flaky on CI.")
	if err != nil || edited.Body != "Still flaky on CI." || !edited.Edited() || !edited.CreatedAt.Equal(second.CreatedAt) {
		t.Errorf("UpdateComment = %+v, %v", edited, err)
	}
	if same, err := s.UpdateComment(ctx, task.Ref, 1, first.Body); err != nil || same.Edited() {
		t.Errorf("unchanged body counts as an edit: %+v, %v", same, err)
	}
	if _, err := s.UpdateComment(ctx, task.Ref, 9, "x"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("update missing comment: err = %v", err)
	}

	// Deleting the newest comment doesn't free its number.
	if deleted, err := s.DeleteComment(ctx, task.Ref, 2); err != nil || deleted.ID != second.ID {
		t.Errorf("DeleteComment = %+v, %v", deleted, err)
	}
	if _, err := s.DeleteComment(ctx, task.Ref, 2); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("delete twice: err = %v", err)
	}
	if third := add(task.Ref, "Third"); third.Number != 3 {
		t.Errorf("number after delete = %d, want 3", third.Number)
	}

	comments, err := s.ListComments(ctx, task.ID)
	if err != nil || len(comments) != 2 || comments[0] != first || comments[1].Number != 3 {
		t.Errorf("ListComments = %+v, %v", comments, err)
	}
	if got, err := s.GetComment(ctx, task.Ref, 3); err != nil || got != comments[1] {
		t.Errorf("GetComment = %+v, %v", got, err)
	}
}
