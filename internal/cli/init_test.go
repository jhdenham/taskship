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
	defer func() { inputIsTerminal = orig }()
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

	r := runInput(t, "\n\nproj\n", "init")
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
	wantErr := "The current folder (~/code/myproject/internal) is part of a repository (~/code/myproject).\n" +
		"Where do you want to set up a Taskship project?\n" +
		"  1) The whole repository (~/code/myproject)\n" +
		"  2) Only this folder (~/code/myproject/internal)\n" +
		"? Enter your choice (1): ? Project name (myproject): ? Task prefix (MYPT): "
	if r.stderr != wantErr {
		t.Errorf("stderr:\n%s\nwant:\n%s", r.stderr, wantErr)
	}

	// The main checkout is registered, and the project is now inferred.
	if out := mustRun(t, "workspace", "list"); !strings.Contains(out, "myproject  ~/code/myproject\n") {
		t.Errorf("workspace list:\n%s", out)
	}
	if out := mustRun(t, "add", "Fix export"); out != "✓ Created PROJ-1: Fix export\n" {
		t.Errorf("add after init = %q", out)
	}

	// Running it again at the root changes nothing.
	t.Chdir(repo)
	r = runInput(t, "", "init")
	want = "Taskship is already set up for this repository.\n  Project: myproject\n  Prefix:  PROJ\nRun \"taskship tasks\" to see its tasks.\n"
	if r.code != 0 || r.stdout != want || r.stderr != "" {
		t.Errorf("init again: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	// In a subfolder, it offers a separate project, defaulting to no.
	t.Chdir(filepath.Join(repo, "internal"))
	r = runInput(t, "\n", "init")
	if r.code != 0 || r.stdout != "This folder is already part of a project (name: myproject, prefix: PROJ).\nRun \"taskship tasks\" to see its tasks.\n" ||
		r.stderr != "? Set up a separate project for this folder? (y/N): " {
		t.Errorf("init in subfolder: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	r = runInput(t, "y\n\n\n", "init")
	if r.code != 0 || !strings.Contains(r.stdout, "Setting up Taskship for ~/code/myproject/internal\nTaskship doesn't write anything to this folder.") ||
		!strings.Contains(r.stdout, "✓ Created project internal. Tasks will be numbered INTL-1") {
		t.Errorf("separate project: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	if out := mustRun(t, "add", "in internal"); out != "✓ Created INTL-1: in internal\n" {
		t.Errorf("add in separate project = %q", out)
	}
}

func TestInitInWorktreeSubfolder(t *testing.T) {
	home := initHome(t)
	repo, worktree := filepath.Join(home, "code", "mono"), filepath.Join(home, "wt", "feature")
	newRepo(t, repo)
	if out, err := exec.Command("git", "-C", repo, "worktree", "add", "-q", worktree).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	mkdirs(t, filepath.Join(repo, "api"), filepath.Join(worktree, "api"))

	// -y picks the whole repository.
	t.Chdir(filepath.Join(worktree, "api"))
	if out := mustRun(t, "init", "-y"); !strings.HasPrefix(out, "Setting up Taskship for ~/code/mono\n") {
		t.Errorf("init -y in worktree subfolder:\n%s", out)
	}
	// Only this folder records its position in the main checkout.
	r := runInput(t, "y\nmono-api\n\n", "init")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "This folder is already part of a project (name: mono, prefix: MONO).\nSetting up Taskship for ~/code/mono/api\n") {
		t.Errorf("separate project from worktree: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	if out := mustRun(t, "workspace", "list"); !strings.Contains(out, "mono-api  ~/code/mono/api\n") {
		t.Errorf("workspace list:\n%s", out)
	}
}

func TestInitInSubfolderOfPlainFolder(t *testing.T) {
	home := initHome(t)
	mkdirs(t, filepath.Join(home, "notes", "2026"))
	mustRun(t, "init", filepath.Join(home, "notes"), "--name", "my-notes", "--prefix", "NOTE")
	t.Chdir(filepath.Join(home, "notes", "2026"))
	if out := mustRun(t, "init", "-y"); out != "This folder is already part of a project (name: my-notes, prefix: NOTE).\nRun \"taskship tasks\" to see its tasks.\n" {
		t.Errorf("init -y in subfolder of a plain folder = %q", out)
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

	r := runInput(t, "\n9\n0\n1\n\n\n", "init")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := "Looks like ~/git contains multiple project repositories.\n" +
		"? Set up ~/git as one project? (y/N): " +
		"Which repository do you want to set up?\n" +
		"  1) newer  changed just now\n" +
		"  2) older  changed 2 days ago\n" +
		"  3) Another folder...\n" +
		"? Enter your choice (1-3):   ✗ Choose a number from 1 to 3.\n" +
		"? Enter your choice (1-3):   ✗ Choose a number from 1 to 3.\n" +
		"? Enter your choice (1-3): " +
		"? Project name (newer): ? Task prefix (NEWR): "
	if r.stderr != want {
		t.Errorf("stderr:\n%s\nwant:\n%s", r.stderr, want)
	}
	if !strings.HasPrefix(r.stdout, "Setting up Taskship for ~/git/newer\n") {
		t.Errorf("stdout:\n%s", r.stdout)
	}

	// "Another folder..." asks for a path; "~" works.
	r = runInput(t, "\n2\n~/nope\n~/git/older\n\n\n", "init")
	if r.code != 0 || !strings.Contains(r.stderr, "? Folder path:   ✗ No such folder: ~/nope.\n? Folder path: ") ||
		!strings.HasPrefix(r.stdout, "Setting up Taskship for ~/git/older\n") {
		t.Errorf("another folder: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	if r := runInput(t, "", "init", "-y"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "pass the one to set up") {
		t.Errorf("folder of repositories with -y: exit %d, stderr %q", r.code, r.stderr)
	}

	// With every repository set up, there is nothing to choose.
	r = runInput(t, "\n", "init")
	if r.code != 0 || r.stdout != "All 3 repositories in ~/git are already set up.\n" {
		t.Errorf("all set up: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	// The folder itself, on request.
	r = runInput(t, "y\n\nPLAIN\n", "init")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "Setting up Taskship for ~/git\nTaskship doesn't write anything to this folder.") {
		t.Errorf("folder itself: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
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

func TestAddOffersSetupWithoutProjects(t *testing.T) {
	home := initHome(t)
	repo := filepath.Join(home, "code", "myproject")
	newRepo(t, repo)
	t.Chdir(repo)

	// Declining adds nothing.
	r := runInput(t, "n\n", "add", "Fix export")
	if r.code != exitFailure || r.stderr != "? Set up Taskship now? (Y/n): taskship: no task added; run \"taskship init\" to set up this repository, or pass --project NAME\n" {
		t.Errorf("declined: exit %d, stderr %q", r.code, r.stderr)
	}
	// Without a terminal, it doesn't ask.
	if r := run(t, "", "add", "Fix export"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "isn't part of a Taskship project. Run \"taskship init\"") {
		t.Errorf("no terminal: exit %d, stderr %q", r.code, r.stderr)
	}
	if out := mustRun(t, "project", "list"); out != "No projects yet. Run \"taskship init\" in a repository to set one up.\n" {
		t.Errorf("project list = %q", out)
	}
	if out := mustRun(t, "tasks"); out != "No projects yet. Run \"taskship init\" in a repository to set one up.\n" {
		t.Errorf("tasks = %q", out)
	}

	r = runInput(t, "\n\n\n", "add", "Fix export")
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	want := "This repository isn't set up with Taskship yet.\n" +
		"Taskship doesn't write anything to this repository. Everything lives in ~/.taskship.\n" +
		"\n" +
		"✓ Created project myproject.\n" +
		"✓ Created MYPT-1: Fix export\n" +
		"Run \"taskship tasks\" to list this project's tasks.\n"
	if r.stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", r.stdout, want)
	}
	if want := "? Set up Taskship now? (Y/n): ? Project name (myproject): ? Task prefix (MYPT): "; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}

	// With a project, an unknown folder is an error, not an offer.
	t.Chdir(home)
	if r := runInput(t, "", "add", "x"); r.code != exitInvalidInput || r.stderr != "taskship: this folder isn't part of a Taskship project. Run \"taskship init\" to set it up, or pass --project NAME to use an existing project.\n" {
		t.Errorf("unknown folder: exit %d, stderr %q", r.code, r.stderr)
	}
}

func TestRunMissingFolderMessage(t *testing.T) {
	home := initHome(t)
	gone := filepath.Join(home, "code", "old")
	mkdirs(t, gone)
	mustRun(t, "init", gone, "--name", "myproject", "--prefix", "PROJ")
	mustRun(t, "add", "x", "-p", "myproject")
	os.RemoveAll(gone)
	t.Chdir(home)
	if r := run(t, "", "PROJ-1", "--dry-run"); r.code != exitNotFound || r.stderr != "taskship: myproject's folder ~/code/old no longer exists. If it moved, run \"taskship init\" in the new location to relink it.\n" {
		t.Errorf("exit %d, stderr %q", r.code, r.stderr)
	}
}
