package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestProjectRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := openTest(t, dir)

	created, err := s.CreateProject(ctx, core.Project{Name: "myproject", TaskPrefix: "PROJ", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Errorf("created project missing ID or timestamps: %+v", created)
	}

	s.Close()
	s = openTest(t, dir)
	got, err := s.GetProject(ctx, "myproject")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Name != "myproject" || got.TaskPrefix != "PROJ" ||
		got.Description != "d" || !got.CreatedAt.Equal(created.CreatedAt) || got.ArchivedAt != nil {
		t.Errorf("GetProject = %+v, want %+v", got, created)
	}
}

func TestCreateProjectRejectsDuplicatesAndInvalid(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	if _, err := s.CreateProject(ctx, core.Project{Name: "myproject", TaskPrefix: "PROJ"}); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]core.Project{
		"duplicate name":   {Name: "myproject", TaskPrefix: "OTHER"},
		"duplicate prefix": {Name: "other", TaskPrefix: "PROJ"},
		"invalid name":     {Name: "Bad Name", TaskPrefix: "BAD"},
	} {
		if _, err := s.CreateProject(ctx, p); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestGetProjectNotFound(t *testing.T) {
	s := openTest(t, t.TempDir())
	if _, err := s.GetProject(context.Background(), "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListProjectsSortedAndEmpty(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	got, err := s.ListProjects(ctx, false)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty list = %#v, %v; want non-nil empty slice", got, err)
	}
	for _, p := range []core.Project{
		{Name: "zeta", TaskPrefix: "ZET"},
		{Name: "alpha", TaskPrefix: "ALP"},
	} {
		if _, err := s.CreateProject(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.ListProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Errorf("ListProjects order = %v", got)
	}
}
