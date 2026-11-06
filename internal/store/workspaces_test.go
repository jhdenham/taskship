package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestWorkspaces(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)

	a, err := s.AddWorkspace(ctx, "myproject", core.Workspace{Path: "/code/myproject", GitCommonDir: "/code/myproject/.git"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == "" || a.Project != "myproject" || a.ProjectID == "" || a.CreatedAt.IsZero() {
		t.Errorf("added = %+v", a)
	}
	// A project can have several workspaces; a path belongs to one project.
	if _, err := s.AddWorkspace(ctx, "myproject", core.Workspace{Path: "/code/myproject-2", GitCommonDir: "/code/myproject/.git"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddWorkspace(ctx, "other", core.Workspace{Path: "/notes"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddWorkspace(ctx, "other", core.Workspace{Path: "/code/myproject"}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("duplicate path: err = %v, want ErrInvalid", err)
	}
	if _, err := s.AddWorkspace(ctx, "nope", core.Workspace{Path: "/x"}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown project: err = %v, want ErrNotFound", err)
	}

	all, err := s.ListWorkspaces(ctx, "")
	if err != nil || len(all) != 3 || all[0] != a || all[2].Path != "/notes" {
		t.Errorf("ListWorkspaces = %+v, %v", all, err)
	}
	if mine, _ := s.ListWorkspaces(ctx, "myproject"); len(mine) != 2 {
		t.Errorf("myproject workspaces = %+v", mine)
	}
	if _, err := s.ListWorkspaces(ctx, "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("list unknown project: err = %v", err)
	}

	removed, err := s.RemoveWorkspace(ctx, "/code/myproject")
	if err != nil || removed != a {
		t.Errorf("RemoveWorkspace = %+v, %v", removed, err)
	}
	if _, err := s.RemoveWorkspace(ctx, "/code/myproject"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("remove twice: err = %v, want ErrNotFound", err)
	}
	if all, _ := s.ListWorkspaces(ctx, ""); len(all) != 2 {
		t.Errorf("after remove = %+v", all)
	}
}

func TestCreateProjectWithWorkspace(t *testing.T) {
	ctx := context.Background()
	s := openTest(t, t.TempDir())
	seedProjects(t, s)
	if _, err := s.AddWorkspace(ctx, "other", core.Workspace{Path: "/notes"}); err != nil {
		t.Fatal(err)
	}

	p, w, err := s.CreateProjectWithWorkspace(ctx, core.Project{Name: "new", TaskPrefix: "NEW"}, core.Workspace{Path: "/code/new", GitCommonDir: "/code/new/.git"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" || w.ProjectID != p.ID || w.Project != "new" || w.Path != "/code/new" {
		t.Errorf("created %+v, %+v", p, w)
	}
	if ws, _ := s.ListWorkspaces(ctx, "new"); len(ws) != 1 {
		t.Errorf("workspaces of new = %+v", ws)
	}

	// Either both are created or neither is.
	for name, tc := range map[string]struct {
		p core.Project
		w core.Workspace
	}{
		"path taken":   {core.Project{Name: "a", TaskPrefix: "A"}, core.Workspace{Path: "/notes"}},
		"name taken":   {core.Project{Name: "new", TaskPrefix: "B"}, core.Workspace{Path: "/b"}},
		"invalid name": {core.Project{Name: "Bad Name", TaskPrefix: "C"}, core.Workspace{Path: "/c"}},
		"prefix taken": {core.Project{Name: "d", TaskPrefix: "NEW"}, core.Workspace{Path: "/d"}},
	} {
		if _, _, err := s.CreateProjectWithWorkspace(ctx, tc.p, tc.w); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
		if _, err := s.GetProject(ctx, tc.p.Name); tc.p.Name != "new" && !errors.Is(err, core.ErrNotFound) {
			t.Errorf("%s: project was created: %v", name, err)
		}
	}
	if ws, _ := s.ListWorkspaces(ctx, ""); len(ws) != 2 {
		t.Errorf("workspaces after failures = %+v", ws)
	}
}
