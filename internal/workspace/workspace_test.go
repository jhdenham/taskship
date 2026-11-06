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
		// Without its position in the main checkout, a worktree of a repository
		// registered for several projects is ambiguous.
		{"monorepo worktree elsewhere", "/worktrees/mono/feature", "/code/mono/.git", "", true},
		{"non-git most specific", "/docs/notes/2026", "", "notes", false},
		{"non-git parent", "/docs/other", "", "everything", false},
		{"unregistered repo", "/code/unknown", "/code/unknown/.git", "", false},
		// A plain folder doesn't claim Git repositories inside it, including
		// itself once it becomes one.
		{"unregistered repo inside registered dir", "/docs/repo", "/docs/repo/.git", "", false},
		{"registered dir turned into a repo", "/docs/notes", "/docs/notes/.git", "", false},
		{"sibling with shared prefix", "/code/myproject-2", "", "", false},
		{"outside everything", "/tmp", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, ok, err := Resolve(Location{Path: tt.path, GitCommonDir: tt.commonDir}, ws)
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

// In a linked worktree, a folder matches by its position in the main
// checkout too, so worktrees of a repository split into several projects
// resolve like the main checkout does.
func TestResolveWorktreeByPosition(t *testing.T) {
	ws := []core.Workspace{
		{Project: "mono-web", Path: "/code/mono", GitCommonDir: "/code/mono/.git"},
		{Project: "mono-api", Path: "/code/mono/api", GitCommonDir: "/code/mono/.git"},
		{Project: "solo", Path: "/worktrees/solo", GitCommonDir: "/code/solo/.git"},
	}
	for _, tt := range []struct{ path, mainPath, want string }{
		{"/worktrees/mono", "/code/mono", "mono-web"},
		{"/worktrees/mono/web", "/code/mono/web", "mono-web"},
		{"/worktrees/mono/api/handlers", "/code/mono/api/handlers", "mono-api"},
		// A registered worktree still matches by its own path.
		{"/worktrees/solo/x", "/code/solo/x", "solo"},
	} {
		commonDir := "/code/mono/.git"
		if tt.want == "solo" {
			commonDir = "/code/solo/.git"
		}
		w, ok, err := Resolve(Location{Path: tt.path, GitCommonDir: commonDir, MainPath: tt.mainPath}, ws)
		if err != nil || !ok || w.Project != tt.want {
			t.Errorf("Resolve(%s) = %q, %v, %v; want %q", tt.path, w.Project, ok, err, tt.want)
		}
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
		loc, err := Inspect(ctx, dir)
		if err != nil || loc.GitCommonDir != want {
			t.Errorf("Inspect(%s) common dir = %q, %v; want %q", dir, loc.GitCommonDir, err, want)
		}
	}
	if loc, _ := Inspect(ctx, filepath.Join(root, "link")); loc.Path != repo {
		t.Errorf("symlink path = %q, want %q", loc.Path, repo)
	}
	if loc, err := Inspect(ctx, plain); err != nil || loc != (Location{Path: plain}) {
		t.Errorf("Inspect(plain) = %+v, %v", loc, err)
	}
	// A folder in a linked worktree knows its position in the main checkout.
	if err := os.MkdirAll(filepath.Join(worktree, "x", "y"), 0o755); err != nil {
		t.Fatal(err)
	}
	if loc, _ := Inspect(ctx, filepath.Join(worktree, "x", "y")); loc.MainPath != filepath.Join(repo, "x", "y") {
		t.Errorf("worktree MainPath = %q", loc.MainPath)
	}
	if loc, _ := Inspect(ctx, nested); loc.MainPath != "" {
		t.Errorf("main checkout MainPath = %q, want none", loc.MainPath)
	}
	if _, err := Inspect(ctx, filepath.Join(root, "missing")); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("missing dir: err = %v, want ErrInvalid", err)
	}

	// Deleting the worktree leaves nothing to clean up: the repository's
	// identity is unchanged.
	git(t, repo, "worktree", "remove", worktree)
	if loc, _ := Inspect(ctx, repo); loc.GitCommonDir != want {
		t.Errorf("after worktree removal common dir = %q", loc.GitCommonDir)
	}
}

func TestForTask(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	base, _ := filepath.EvalSymlinks(t.TempDir())
	p := func(rel string) string { return filepath.Join(base, rel) }
	mkdir := func(rels ...string) {
		for _, rel := range rels {
			if err := os.MkdirAll(p(rel), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	mkdir("myproject/internal/cli", "mono/api/handlers", "mono/web", "solo", "notes", "notes-2/sub")
	for _, f := range []string{"mono/api/.keep", "mono/web/.keep"} {
		if err := os.WriteFile(p(f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, repo := range []string{"myproject", "mono", "solo"} {
		git(t, p(repo), "init", "-q")
		git(t, p(repo), "add", ".")
		git(t, p(repo), "commit", "-q", "--allow-empty", "-m", "init")
	}
	git(t, p("myproject"), "worktree", "add", "-q", p("wt/feature"))
	git(t, p("myproject"), "worktree", "add", "-q", p("myproject/.worktrees/inner"))
	git(t, p("mono"), "worktree", "add", "-q", p("wt/mono"))
	git(t, p("solo"), "worktree", "add", "-q", p("wt/solo-registered"))
	git(t, base, "clone", "-q", "--bare", p("solo"), p("bare/.bare"))
	git(t, p("bare/.bare"), "worktree", "add", "-q", p("bare/main"))
	mkdir("wt/feature/docs")

	ws := []core.Workspace{
		{ProjectID: "p1", Project: "myproject", Path: p("myproject")},
		{ProjectID: "p2", Project: "mono-web", Path: p("mono")},
		{ProjectID: "p3", Project: "mono-api", Path: p("mono/api")},
		{ProjectID: "p4", Project: "notes", Path: p("notes")},
		{ProjectID: "p4", Project: "notes", Path: p("notes-2")},
		{ProjectID: "p6", Project: "gone", Path: p("missing")},
		{ProjectID: "p7", Project: "solo", Path: p("wt/solo-registered")},
		{ProjectID: "p8", Project: "bare", Path: p("bare/.bare")},
	}
	for i, w := range ws {
		if loc, _ := Inspect(ctx, w.Path); w.Project != "gone" {
			ws[i].GitCommonDir = loc.GitCommonDir
		} else {
			ws[i].GitCommonDir = ""
		}
	}
	// A registered linked worktree that is later deleted.
	git(t, p("solo"), "worktree", "remove", p("wt/solo-registered"))

	task := func(id, name string) core.Task { return core.Task{ProjectID: id, Project: name} }
	tests := []struct {
		name     string
		task     core.Task
		dir      string
		explicit bool
		want     string
		err      error
	}{
		{"cwd in checkout subfolder", task("p1", "myproject"), "myproject/internal/cli", false, "myproject", nil},
		{"cwd in linked worktree subfolder", task("p1", "myproject"), "wt/feature/docs", false, "wt/feature", nil},
		{"cwd in worktree nested in checkout", task("p1", "myproject"), "myproject/.worktrees/inner", false, "myproject/.worktrees/inner", nil},
		{"cwd elsewhere uses sole workspace", task("p1", "myproject"), "notes-2/sub", false, "myproject", nil},
		{"cwd in other project uses sole workspace", task("p1", "myproject"), "notes", false, "myproject", nil},
		{"monorepo subfolder project", task("p3", "mono-api"), "mono/api/handlers", false, "mono/api", nil},
		{"monorepo sibling folder is not claimed", task("p3", "mono-api"), "mono/web", false, "mono/api", nil},
		{"monorepo worktree, subfolder project", task("p3", "mono-api"), "wt/mono/web", false, "wt/mono/api", nil},
		{"monorepo worktree, root project", task("p2", "mono-web"), "wt/mono/api", false, "wt/mono", nil},
		{"registered worktree deleted", task("p7", "solo"), "solo", false, "solo", nil},
		{"worktree of a registered bare repo", task("p8", "bare"), "bare/main", false, "bare/main", nil},
		{"bare repo from outside its worktrees", task("p8", "bare"), "notes", false, "", core.ErrInvalid},
		{"several workspaces", task("p4", "notes"), "myproject", false, "", core.ErrAmbiguous},
		{"no workspace", task("p5", "empty"), "notes", false, "", core.ErrNotFound},
		{"sole workspace missing", task("p6", "gone"), "notes", false, "", core.ErrNotFound},
		{"explicit subfolder", task("p1", "myproject"), "myproject/internal", true, "myproject", nil},
		{"explicit worktree", task("p1", "myproject"), "wt/feature/docs", true, "wt/feature", nil},
		{"explicit one of several", task("p4", "notes"), "notes-2/sub", true, "notes-2", nil},
		{"explicit other project", task("p1", "myproject"), "notes", true, "", core.ErrInvalid},
		// In the same repository, the project's folder in that checkout is used.
		{"explicit other project's folder in the repo", task("p3", "mono-api"), "mono/web", true, "mono/api", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := Inspect(ctx, p(tt.dir))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ForTask(ctx, tt.task, loc, tt.explicit, ws)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Errorf("err = %v, want %v", err, tt.err)
				}
				return
			}
			if err != nil || got != p(tt.want) {
				t.Errorf("ForTask = %q, %v; want %q", got, err, p(tt.want))
			}
		})
	}
}

func TestMainCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	base, _ := filepath.EvalSymlinks(t.TempDir())
	p := func(rel string) string { return filepath.Join(base, rel) }
	if err := os.MkdirAll(p("myproject/internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p("plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, p("myproject"), "init", "-q")
	git(t, p("myproject"), "commit", "-q", "--allow-empty", "-m", "init")
	git(t, p("myproject"), "worktree", "add", "-q", p("wt/feature"))
	// A bare repository with worktrees next to it, a common layout for
	// worktree-heavy workflows.
	git(t, base, "clone", "-q", "--bare", p("myproject"), p("other/.bare"))
	git(t, p("other/.bare"), "worktree", "add", "-q", p("other/main"))

	tests := []struct {
		dir, want string
		bare      bool
	}{
		{"myproject", "myproject", false},
		{"myproject/internal", "myproject", false},
		{"wt/feature", "myproject", false},
		{"other/main", "other/.bare", true},
	}
	for _, tt := range tests {
		got, bare := MainCheckout(ctx, p(tt.dir))
		if got != p(tt.want) || bare != tt.bare {
			t.Errorf("MainCheckout(%s) = %q, %v; want %q, %v", tt.dir, got, bare, p(tt.want), tt.bare)
		}
	}
	if got, _ := MainCheckout(ctx, p("plain")); got != "" {
		t.Errorf("MainCheckout(plain) = %q, want none", got)
	}
}

func TestRemoteName(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	worktree := filepath.Join(t.TempDir(), "feature")
	git(t, repo, "worktree", "add", "-q", worktree)

	if got := RemoteName(ctx, repo); got != "" {
		t.Errorf("no remotes: %q", got)
	}
	git(t, repo, "remote", "add", "upstream", "https://github.com/someone/upstream-name.git/")
	if got := RemoteName(ctx, repo); got != "upstream-name" {
		t.Errorf("one non-origin remote: %q", got)
	}
	git(t, repo, "remote", "add", "fork", "https://github.com/me/fork-name")
	if got := RemoteName(ctx, repo); got != "" {
		t.Errorf("two remotes, no origin: %q", got)
	}
	git(t, repo, "remote", "add", "origin", "git@github.com:me/myproject.git")
	for _, dir := range []string{repo, worktree} {
		if got := RemoteName(ctx, dir); got != "myproject" {
			t.Errorf("origin from %s: %q", dir, got)
		}
	}
	if got := RemoteName(ctx, t.TempDir()); got != "" {
		t.Errorf("not a repository: %q", got)
	}
}

func TestRepoNameFromURL(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:me/myproject.git":             "myproject",
		"git@github.com:myproject.git":                "myproject",
		"https://github.com/me/myproject":             "myproject",
		"https://github.com/me/myproject.git/":        "myproject",
		"ssh://git@example.com:2222/me/MyProject.git": "MyProject",
		"/srv/git/myproject.git":                      "myproject",
		"file:///srv/git/myproject":                   "myproject",
		"myproject":                                   "myproject",
		"":                                            "",
	} {
		if got := repoNameFromURL(url); got != want {
			t.Errorf("repoNameFromURL(%q) = %q, want %q", url, got, want)
		}
	}
}
