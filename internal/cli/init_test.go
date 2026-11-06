package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// initHome points HOME at a temp folder, with Taskship's data in
// ~/.taskship, so output shows ~/ paths. It returns the home folder.
func initHome(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	home, _ := filepath.EvalSymlinks(t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("TASKSHIP_HOME", filepath.Join(home, ".taskship"))
	return home
}

// newRepo creates a Git repository with one commit at path.
func newRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", path}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// runInput runs a command at a terminal, typing input.
func runInput(t *testing.T, input string, args ...string) result {
	t.Helper()
	orig := inputIsTerminal
	t.Cleanup(func() { inputIsTerminal = orig })
	inputIsTerminal = func(io.Reader) bool { return true }
	cmd := NewRootCmd("")
	var out, errb bytes.Buffer
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	code := Execute(cmd, args)
	return result{out.String(), errb.String(), code}
}

const nextSteps = `Next steps:
  taskship add "Fix the export bug"   Add a task from anywhere in this repository
  taskship tasks                      List this project's tasks
  taskship show PROJ-1                Show a task
  taskship PROJ-1 --dry-run           Preview what an agent would be given
`

func TestInitInRepository(t *testing.T) {
	home := initHome(t)
	repo := filepath.Join(home, "code", "myproject")
	newRepo(t, repo)
	mkdirs(t, filepath.Join(repo, "internal"))
	t.Chdir(filepath.Join(repo, "internal"))

	r := runInput(t, "\nproj\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := `Setting up Taskship for ~/code/myproject
Taskship doesn't write anything to this repository. Everything lives in ~/.taskship.


✓ Created project myproject. Tasks will be numbered PROJ-1, PROJ-2, and so on.

` + nextSteps
	if r.stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", r.stdout, want)
	}
	if want := "? Project name (myproject): ? Task prefix (MYPT): "; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}

	// The main checkout is registered, and the project is now inferred.
	if out := mustRun(t, "workspace", "list"); !strings.Contains(out, "myproject  ~/code/myproject\n") {
		t.Errorf("workspace list:\n%s", out)
	}
	if out := mustRun(t, "add", "Fix export"); out != "✓ Created PROJ-1: Fix export\n" {
		t.Errorf("add after init = %q", out)
	}

	// Running it again changes nothing.
	r = runInput(t, "", "init")
	want = "Taskship is already set up for this repository.\n  Project: myproject\n  Prefix:  PROJ\nRun \"taskship tasks\" to see its tasks.\n"
	if r.code != 0 || r.stdout != want || r.stderr != "" {
		t.Errorf("init again: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestInitAcceptsDefaultsWithY(t *testing.T) {
	home := initHome(t)
	repo := filepath.Join(home, "oldname")
	newRepo(t, repo)
	// The remote's name wins over a stale folder name.
	if err := exec.Command("git", "-C", repo, "remote", "add", "origin", "git@github.com:me/task-ship.git").Run(); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	out := mustRun(t, "init", "-y")
	want := "Setting up Taskship for ~/oldname\n" +
		"Taskship doesn't write anything to this repository. Everything lives in ~/.taskship.\n\n" +
		"✓ Created project task-ship. Tasks will be numbered TS-1, TS-2, and so on.\n\n"
	if !strings.HasPrefix(out, want) || !strings.Contains(out, "taskship TS-1 --dry-run") {
		t.Errorf("init -y:\n%s", out)
	}
}

func TestInitWithFlagsNeedsNoTerminal(t *testing.T) {
	home := initHome(t)
	t.Chdir(home)
	mkdirs(t, filepath.Join(home, "notes"))
	out := mustRun(t, "init", filepath.Join(home, "notes"), "--name", "my-notes", "--prefix", "note")
	if !strings.HasPrefix(out, "Setting up Taskship for ~/notes\nTaskship doesn't write anything to this folder.") ||
		!strings.Contains(out, "✓ Created project my-notes. Tasks will be numbered NOTE-1") ||
		!strings.Contains(out, "Add a task from anywhere in this folder") {
		t.Errorf("init with flags:\n%s", out)
	}
}

func TestInitErrors(t *testing.T) {
	home := initHome(t)
	repo := filepath.Join(home, "myproject")
	newRepo(t, repo)

	t.Chdir(repo)
	if r := run(t, "", "init"); r.code != exitInvalidInput || r.stderr != "taskship: init needs a terminal to ask questions; pass -y to accept the defaults, or --name and --prefix\n" {
		t.Errorf("no terminal: exit %d, stderr %q", r.code, r.stderr)
	}
	t.Chdir(home)
	if r := runInput(t, "", "init"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "not your home folder, or pass one: taskship init ~/code/myproject") {
		t.Errorf("home: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := runInput(t, "", "init", "/"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "not the filesystem root") {
		t.Errorf("root: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := runInput(t, "", "init", "-y", filepath.Join(home, "2024"), "--prefix", "X"); r.code != exitInvalidInput {
		t.Errorf("missing folder: exit %d, stderr %q", r.code, r.stderr)
	}
	mkdirs(t, filepath.Join(home, "2024"))
	if r := run(t, "", "init", "-y", filepath.Join(home, "2024")); r.code != exitInvalidInput || !strings.Contains(r.stderr, "pass --prefix") {
		t.Errorf("no default prefix: exit %d, stderr %q", r.code, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".taskship")); err == nil {
		if out := mustRun(t, "project", "list"); !strings.Contains(out, "No projects") {
			t.Errorf("errors created projects:\n%s", out)
		}
	}
}

func TestInitRetriesInvalidAnswers(t *testing.T) {
	home := initHome(t)
	mkdirs(t, filepath.Join(home, "plain"), filepath.Join(home, "notes"))
	t.Chdir(filepath.Join(home, "plain"))
	mustRun(t, "project", "add", "other", "--prefix", "MP")

	r := runInput(t, "My Project\n\nmp\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := "? Project name (plain): " +
		"  ✗ Names use lowercase letters, digits, and hyphens.\n" +
		"? Project name (my-project): " +
		"? Task prefix (MP2): " +
		"  ✗ MP is already used by project other.\n" +
		"? Task prefix (MP2): "
	if r.stderr != want {
		t.Errorf("stderr:\n%q\nwant:\n%q", r.stderr, want)
	}
	if !strings.Contains(r.stdout, "✓ Created project my-project. Tasks will be numbered MP2-1") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
}

func TestInitLinksExistingProject(t *testing.T) {
	home := initHome(t)
	repo := filepath.Join(home, "code", "myproject")
	newRepo(t, repo)
	gone := filepath.Join(home, "old-clone")
	mkdirs(t, gone)
	mustRun(t, "project", "add", "myproject", "--prefix", "PROJ")
	mustRun(t, "workspace", "add", "myproject", "--path", gone)
	os.RemoveAll(gone)

	t.Chdir(repo)
	r := runInput(t, "\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if want := "? Project name (myproject): ? myproject already exists (prefix PROJ). Link this folder to it? (Y/n): "; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
	if !strings.Contains(r.stdout, "\n\n✓ Linked ~/code/myproject to myproject.\n\n"+nextSteps) {
		t.Errorf("stdout:\n%s", r.stdout)
	}
	// The dead clone was replaced, not kept alongside.
	if out := mustRun(t, "workspace", "list"); strings.Contains(out, "old-clone") || !strings.Contains(out, "~/code/myproject") {
		t.Errorf("workspace list:\n%s", out)
	}
}

func TestInitDeclinedLinkAsksForAnotherName(t *testing.T) {
	home := initHome(t)
	repo := filepath.Join(home, "myproject")
	newRepo(t, repo)
	mustRun(t, "project", "add", "myproject", "--prefix", "PROJ")
	t.Chdir(repo)

	r := runInput(t, "\nn\nfresh\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if want := "? Project name (myproject): ? myproject already exists (prefix PROJ). Link this folder to it? (Y/n): ? Project name: ? Task prefix (FRSH): "; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
	if !strings.Contains(r.stdout, "✓ Created project fresh. Tasks will be numbered FRSH-1") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
}

func TestInitChoosesRepositoryInFolder(t *testing.T) {
	home := initHome(t)
	gitDir := filepath.Join(home, "git")
	for _, name := range []string{"older", "newer", "done"} {
		newRepo(t, filepath.Join(gitDir, name))
	}
	mkdirs(t, filepath.Join(gitDir, "not-a-repo"))
	old := time.Now().Add(-50 * time.Hour)
	os.Chtimes(filepath.Join(gitDir, "older", ".git"), old, old)
	mustRun(t, "project", "add", "done", "--prefix", "DONE")
	mustRun(t, "workspace", "add", "done", "--path", filepath.Join(gitDir, "done"))
	t.Chdir(gitDir)

	r := runInput(t, "9\n0\n1\n\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := "~/git isn't a Git repository, but it contains 2 that aren't set up yet:\n" +
		"  1) newer  changed just now\n" +
		"  2) older  changed 2 days ago\n" +
		"  3) Another folder...\n" +
		"  4) ~/git itself\n" +
		"? Which one do you want to set up? (1-4):   ✗ Choose a number from 1 to 4.\n" +
		"? Which one do you want to set up? (1-4):   ✗ Choose a number from 1 to 4.\n" +
		"? Which one do you want to set up? (1-4): " +
		"? Project name (newer): ? Task prefix (NEWR): "
	if r.stderr != want {
		t.Errorf("stderr:\n%s\nwant:\n%s", r.stderr, want)
	}
	if !strings.HasPrefix(r.stdout, "Setting up Taskship for ~/git/newer\n") {
		t.Errorf("stdout:\n%s", r.stdout)
	}

	// "Another folder..." asks for a path; "~" works.
	r = runInput(t, "2\n~/nope\n~/git/older\n\n\n", "init")
	if r.code != 0 || !strings.Contains(r.stderr, "? Folder path:   ✗ No such folder: ~/nope.\n? Folder path: ") ||
		!strings.HasPrefix(r.stdout, "Setting up Taskship for ~/git/older\n") {
		t.Errorf("another folder: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	if r := runInput(t, "", "init", "-y"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "pass the one to set up") {
		t.Errorf("chooser with -y: exit %d, stderr %q", r.code, r.stderr)
	}

	// With every repository set up, only the other choices are left.
	r = runInput(t, "2\n\nPLAIN\n", "init")
	if r.code != 0 || !strings.HasPrefix(r.stderr, "All 3 repositories in ~/git are already set up.\n  1) Another folder...\n  2) ~/git itself\n") ||
		!strings.HasPrefix(r.stdout, "Setting up Taskship for ~/git\nTaskship doesn't write anything to this folder.") {
		t.Errorf("all set up: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	// Now ~/git itself is set up too.
	if r := runInput(t, "", "init"); r.code != 0 || !strings.HasPrefix(r.stdout, "Taskship is already set up for this folder.\n  Project: git\n") {
		t.Errorf("init in ~/git again: exit %d, stdout %q", r.code, r.stdout)
	}
}

func TestInitBareRepository(t *testing.T) {
	home := initHome(t)
	src := filepath.Join(home, "src")
	newRepo(t, src)
	bare, main := filepath.Join(home, "other", ".bare"), filepath.Join(home, "other", "main")
	for _, args := range [][]string{{"clone", "-q", "--bare", src, bare}, {"-C", bare, "remote", "remove", "origin"}, {"-C", bare, "worktree", "add", "-q", main}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(main)
	out := mustRun(t, "init", "-y")
	if !strings.HasPrefix(out, "Setting up Taskship for ~/other/.bare\n") || !strings.Contains(out, "Created project other.") {
		t.Errorf("init in bare worktree:\n%s", out)
	}
	mustRun(t, "add", "x")
	if out := mustRun(t, "run", "OTHR-1", "--dry-run"); !strings.Contains(out, "Workspace: ~/other/main\n") {
		t.Errorf("run from the worktree:\n%s", out)
	}
	t.Chdir(home)
	if r := run(t, "", "OTHR-1", "--dry-run"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "bare repository") {
		t.Errorf("run outside worktrees: exit %d, stderr %q", r.code, r.stderr)
	}
}

func TestChangedAgo(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.Local)
	for d, want := range map[time.Duration]string{
		10 * time.Second:     "changed just now",
		time.Minute:          "changed 1 minute ago",
		5 * time.Minute:      "changed 5 minutes ago",
		3 * time.Hour:        "changed 3 hours ago",
		30 * time.Hour:       "changed yesterday",
		4 * 24 * time.Hour:   "changed 4 days ago",
		60 * 24 * time.Hour:  "changed in September",
		400 * 24 * time.Hour: "changed in October 2025",
	} {
		if got := changedAgo(now.Add(-d), now); got != want {
			t.Errorf("changedAgo(-%v) = %q, want %q", d, got, want)
		}
	}
}
