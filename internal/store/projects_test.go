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

	created, err := s.CreateProject(ctx, core.Project{Slug: "myproject", Name: "My Project", TaskPrefix: "PROJ", Description: "d"})
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
	if got.ID != created.ID || got.Name != "My Project" || got.TaskPrefix != "PROJ" ||
		got.Description != "d" || !got.CreatedAt.Equal(created.CreatedAt) || got.ArchivedAt != nil {
		t.Errorf("GetProject = %+v, want %+v", got, created)
	}
}

func TestCreateProjectRejectsDuplicatesAndInvalid(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	if _, err := s.CreateProject(ctx, core.Project{Slug: "myproject", Name: "myproject", TaskPrefix: "PROJ"}); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]core.Project{
		"duplicate slug":   {Slug: "myproject", Name: "x", TaskPrefix: "OTHER"},
		"duplicate prefix": {Slug: "other", Name: "x", TaskPrefix: "PROJ"},
		"invalid slug":     {Slug: "Bad Slug", Name: "x", TaskPrefix: "BAD"},
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
		{Slug: "zeta", Name: "zeta", TaskPrefix: "ZET"},
		{Slug: "alpha", Name: "alpha", TaskPrefix: "ALP"},
	} {
		if _, err := s.CreateProject(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.ListProjects(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "alpha" || got[1].Slug != "zeta" {
		t.Errorf("ListProjects order = %v", got)
	}
}
