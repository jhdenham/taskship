package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestResolve(t *testing.T) {
	ws := []core.Workspace{
		{Project: "myproject", Path: "/code/myproject", GitCommonDir: "/code/myproject/.git"},
		{Project: "mono-web", Path: "/code/mono", GitCommonDir: "/code/mono/.git"},
		{Project: "mono-api", Path: "/code/mono/api", GitCommonDir: "/code/mono/.git"},
		{Project: "notes", Path: "/docs/notes"},
		{Project: "everything", Path: "/docs"},
	}
	tests := []struct {
		name, path, commonDir string
		want                  string // project, "" for no match
		ambiguous             bool
	}{
		{"checkout root", "/code/myproject", "/code/myproject/.git", "myproject", false},
		{"nested directory", "/code/myproject/internal/cli", "/code/myproject/.git", "myproject", false},
		{"linked worktree elsewhere", "/worktrees/myproject/feature", "/code/myproject/.git", "myproject", false},
		{"monorepo root", "/code/mono/web", "/code/mono/.git", "mono-web", false},
		{"monorepo most specific", "/code/mono/api/handlers", "/code/mono/.git", "mono-api", false},
		{"monorepo worktree elsewhere", "/worktrees/mono/feature", "/code/mono/.git", "", true},
		{"non-git most specific", "/docs/notes/2026", "", "notes", false},
		{"non-git parent", "/docs/other", "", "everything", false},
		{"unregistered repo", "/code/unknown", "/code/unknown/.git", "", false},
		{"unregistered repo inside registered dir", "/docs/repo", "/docs/repo/.git", "everything", false},
		{"sibling with shared prefix", "/code/myproject-2", "", "", false},
		{"outside everything", "/tmp", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, ok, err := Resolve(tt.path, tt.commonDir, ws)
			if tt.ambiguous {
				if !errors.Is(err, core.ErrAmbiguous) {
					t.Errorf("err = %v, want ErrAmbiguous", err)
				}
				return
			}
			if err != nil || ok != (tt.want != "") || w.Project != tt.want {
				t.Errorf("Resolve = %q, %v, %v; want %q", w.Project, ok, err, tt.want)
			}
		})
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestInspect(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	repo := filepath.Join(root, "myproject")
	nested := filepath.Join(repo, "a", "b")
	worktree := filepath.Join(root, "worktrees", "feature")
	plain := filepath.Join(root, "plain")
	for _, d := range []string{nested, plain} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repo, "init", "-q")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	git(t, repo, "worktree", "add", "-q", worktree)
	if err := os.Symlink(repo, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(repo, ".git")
	for _, dir := range []string{repo, nested, worktree, filepath.Join(root, "link")} {
		_, common, err := Inspect(ctx, dir)
		if err != nil || common != want {
			t.Errorf("Inspect(%s) common dir = %q, %v; want %q", dir, common, err, want)
		}
	}
	if path, _, _ := Inspect(ctx, filepath.Join(root, "link")); path != repo {
		t.Errorf("symlink path = %q, want %q", path, repo)
	}
	if path, common, err := Inspect(ctx, plain); err != nil || path != plain || common != "" {
		t.Errorf("Inspect(plain) = %q, %q, %v", path, common, err)
	}
	if _, _, err := Inspect(ctx, filepath.Join(root, "missing")); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("missing dir: err = %v, want ErrInvalid", err)
	}

	// Deleting the worktree leaves nothing to clean up: the repository's
	// identity is unchanged.
	git(t, repo, "worktree", "remove", worktree)
	if _, common, _ := Inspect(ctx, repo); common != want {
		t.Errorf("after worktree removal common dir = %q", common)
	}
}
