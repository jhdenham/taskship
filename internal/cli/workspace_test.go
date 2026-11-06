package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo creates a repository with one commit and a linked worktree, and
// returns their symlink-free paths.
func gitRepo(t *testing.T) (repo, worktree string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root, _ := filepath.EvalSymlinks(t.TempDir())
	repo, worktree = filepath.Join(root, "myproject"), filepath.Join(root, "worktrees", "feature")
	if err := os.MkdirAll(filepath.Join(repo, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
		{"worktree", "add", "-q", worktree},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo, worktree
}

func TestWorkspaceCommands(t *testing.T) {
	seedCLIProjects(t)
	repo, worktree := gitRepo(t)
	notes := t.TempDir()

	out := mustRun(t, "workspace", "add", "myproject", "--path", repo)
	if !strings.HasPrefix(out, "✓ Added workspace "+repo+" for myproject\n") || !strings.Contains(out, "recognized automatically") {
		t.Errorf("add output = %q", out)
	}
	t.Chdir(notes)
	if out := mustRun(t, "workspace", "add", "other"); out != "✓ Added workspace "+mustEval(t, notes)+" for other\n" {
		t.Errorf("add default path output = %q", out)
	}
	if r := run(t, "", "workspace", "add", "other", "--path", repo); r.code != exitInvalidInput {
		t.Errorf("duplicate path: exit %d", r.code)
	}
	if r := run(t, "", "workspace", "add", "other", "--path", filepath.Join(notes, "missing")); r.code != exitInvalidInput {
		t.Errorf("missing path: exit %d", r.code)
	}
	if r := run(t, "", "workspace", "add", "nope", "--path", repo); r.code != exitNotFound {
		t.Errorf("unknown project: exit %d", r.code)
	}

	list := mustRun(t, "workspace", "list")
	if !strings.Contains(list, "myproject  "+repo) || !strings.Contains(list, "other") || strings.Contains(list, "missing") {
		t.Errorf("list:\n%s", list)
	}
	// The worktree is recognized but never stored.
	if strings.Contains(list, worktree) {
		t.Errorf("worktree was stored:\n%s", list)
	}
	if out := mustRun(t, "workspace", "list", "-p", "other"); strings.Contains(out, "myproject") {
		t.Errorf("list -p other:\n%s", out)
	}

	// A deleted directory is reported as missing and can still be removed.
	os.RemoveAll(notes)
	t.Chdir(repo)
	if out := mustRun(t, "workspace", "list"); !strings.Contains(out, "(missing)") {
		t.Errorf("list after delete:\n%s", out)
	}
	if out := mustRun(t, "workspace", "remove", mustEval(t, filepath.Dir(notes))+"/"+filepath.Base(notes)); !strings.HasPrefix(out, "✓ Removed workspace") {
		t.Errorf("remove output = %q", out)
	}
	if r := run(t, "", "workspace", "remove", notes); r.code != exitNotFound {
		t.Errorf("remove twice: exit %d", r.code)
	}
	if out := mustRun(t, "workspace", "remove", "."); out != "✓ Removed workspace "+repo+" from myproject\n" {
		t.Errorf("remove . output = %q", out)
	}
	if out := mustRun(t, "workspace", "list"); !strings.HasPrefix(out, "No workspaces.") {
		t.Errorf("empty list = %q", out)
	}
}

func TestAddInfersProject(t *testing.T) {
	seedCLIProjects(t)
	repo, worktree := gitRepo(t)
	mustRun(t, "workspace", "add", "myproject", "--path", repo)

	for i, dir := range []string{repo, filepath.Join(repo, "internal"), worktree} {
		t.Chdir(dir)
		want := "✓ Created PROJ-" + string(rune('1'+i)) + ": from " + filepath.Base(dir) + "\n"
		if out := mustRun(t, "add", "from "+filepath.Base(dir)); out != want {
			t.Errorf("in %s: %q, want %q", dir, out, want)
		}
	}
	// An explicit --project wins over the directory.
	if out := mustRun(t, "add", "explicit", "-p", "other"); out != "✓ Created OTH-1: explicit\n" {
		t.Errorf("explicit project: %q", out)
	}
	// Deleting the worktree leaves nothing to clean up.
	if out, err := exec.Command("git", "-C", repo, "worktree", "remove", worktree).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Chdir(repo)
	if out := mustRun(t, "workspace", "list"); strings.Count(out, "\n") != 2 || strings.Contains(out, "missing") {
		t.Errorf("list after worktree removal:\n%s", out)
	}

	// The same repository registered for two projects: a worktree matches
	// by its position in the main checkout.
	mustRun(t, "workspace", "add", "other", "--path", filepath.Join(repo, "internal"))
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", worktree).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	t.Chdir(worktree)
	if out := mustRun(t, "add", "x"); !strings.HasPrefix(out, "✓ Created PROJ-") {
		t.Errorf("worktree root: %q", out)
	}
	t.Chdir(filepath.Join(repo, "internal"))
	if out := mustRun(t, "add", "most specific"); !strings.HasPrefix(out, "✓ Created OTH-") {
		t.Errorf("most specific: %q", out)
	}

	t.Chdir(t.TempDir())
	if r := run(t, "", "add", "x"); r.code != exitInvalidInput || r.stderr != "taskship: this folder isn't part of a Taskship project. Run \"taskship init\" to set it up, or pass --project NAME to use an existing project.\n" {
		t.Errorf("unregistered dir: exit %d, stderr %q", r.code, r.stderr)
	}
}

func mustEval(t *testing.T, path string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTasksScope(t *testing.T) {
	seedCLIProjects(t)
	repo, worktree := gitRepo(t)
	mustRun(t, "workspace", "add", "myproject", "--path", repo)
	mustRun(t, "add", "a", "-p", "myproject")
	mustRun(t, "add", "b", "-p", "myproject", "--status", "done")
	mustRun(t, "add", "c", "-p", "other")

	orig := isTerminal
	t.Cleanup(func() { isTerminal = orig })
	isTerminal = func(io.Writer) bool { return true }

	firstLine := func(args ...string) string {
		t.Helper()
		out := mustRun(t, args...)
		line, _, _ := strings.Cut(out, "\n")
		return line
	}
	t.Chdir(worktree)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"tasks"}, "Showing 2 tasks in myproject (this folder). Use --all-projects to see every project."},
		{[]string{"tasks", "--status", "todo"}, "Showing 1 task in myproject (this folder), with status todo. Use --all-projects to see every project."},
		{[]string{"tasks", "-p", "other"}, "Showing 1 task in other. Use --all-projects to see every project."},
		{[]string{"tasks", "--all-projects", "--archived"}, "Showing 3 tasks across all projects, including archived."},
		{[]string{"task", "list", "--status", "canceled"}, "No tasks in myproject (this folder), with status canceled. Use --all-projects to see every project."},
	}
	for _, tt := range tests {
		if got := firstLine(tt.args...); got != tt.want {
			t.Errorf("%v first line =\n  %q\nwant\n  %q", tt.args, got, tt.want)
		}
	}
	if out := mustRun(t, "tasks"); strings.Contains(out, "OTH-1") || !strings.Contains(out, "PROJ-2") {
		t.Errorf("tasks in worktree:\n%s", out)
	}

	t.Chdir(t.TempDir())
	if got := firstLine("tasks"); got != "Showing 3 tasks across all projects. Use --project NAME for one project." {
		t.Errorf("outside any project: %q", got)
	}

	// Piped output has no summary line, and JSON is unaffected.
	isTerminal = func(io.Writer) bool { return false }
	if got := firstLine("tasks"); !strings.HasPrefix(got, "REF") {
		t.Errorf("piped first line = %q", got)
	}
	if r := run(t, "", "tasks", "-p", "other", "--all-projects"); r.code != exitInvalidInput {
		t.Errorf("--project with --all-projects: exit %d", r.code)
	}
}

// In a worktree of a repository split into two projects, a task goes to the
// project of the folder's position in the main checkout.
func TestAddInWorktreeOfSplitRepository(t *testing.T) {
	seedCLIProjects(t)
	repo, worktree := gitRepo(t)
	mustRun(t, "workspace", "add", "myproject", "--path", repo)
	mustRun(t, "workspace", "add", "other", "--path", filepath.Join(repo, "internal"))
	if err := os.MkdirAll(filepath.Join(worktree, "internal", "cli"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join(worktree, "internal", "cli"))
	if out := mustRun(t, "add", "in internal"); out != "✓ Created OTH-1: in internal\n" {
		t.Errorf("add in worktree subfolder = %q", out)
	}
	t.Chdir(worktree)
	if out := mustRun(t, "add", "at the root"); out != "✓ Created PROJ-1: at the root\n" {
		t.Errorf("add at worktree root = %q", out)
	}
}
