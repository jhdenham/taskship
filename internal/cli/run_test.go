package cli

import (
	"os"
	"strings"
	"testing"
)

func TestRunDryRun(t *testing.T) {
	seedCLIProjects(t)
	repo, worktree := gitRepo(t)
	mustRun(t, "add", "Add weekly habit tracker", "-p", "myproject", "--description", "## Acceptance criteria\n\n- [ ] Weekly view")
	mustRun(t, "add", "Elsewhere", "-p", "other")

	// No workspace registered yet: setup guidance.
	if r := run(t, "", "run", "PROJ-1", "--dry-run"); r.code != exitNotFound || !strings.Contains(r.stderr, "taskship workspace add myproject") {
		t.Errorf("no workspace: exit %d, stderr %q", r.code, r.stderr)
	}
	mustRun(t, "workspace", "add", "myproject", "--path", repo)

	// From the worktree, the worktree is used.
	t.Chdir(worktree)
	out := mustRun(t, "run", "proj-1", "--dry-run", "-a", "claude")
	header := "Dry run: nothing was launched.\nAgent:     claude\nWorkspace: " + worktree + "\n\nYou are beginning a Taskship agent run.\n"
	if !strings.HasPrefix(out, header) || !strings.Contains(out, "Task reference: PROJ-1\n") ||
		!strings.Contains(out, "# Add weekly habit tracker\n\n## Acceptance criteria\n\n- [ ] Weekly view\n```\n") {
		t.Errorf("dry run output:\n%s", out)
	}

	// From an unrelated directory, the project's only workspace is used.
	t.Chdir(t.TempDir())
	if out := mustRun(t, "run", "PROJ-1", "--dry-run"); !strings.Contains(out, "Workspace: "+repo+"\n") {
		t.Errorf("sole workspace output:\n%s", out)
	}
	if out := mustRun(t, "run", "PROJ-1", "--dry-run", "--workspace", worktree); !strings.Contains(out, "Workspace: "+worktree+"\n") {
		t.Errorf("--workspace output:\n%s", out)
	}

	for _, tt := range []struct {
		name string
		args []string
		code int
	}{
		{"not a workspace of the project", []string{"run", "PROJ-1", "--dry-run", "--workspace", t.TempDir()}, exitInvalidInput},
		{"missing task", []string{"run", "PROJ-99", "--dry-run"}, exitNotFound},
		{"bad ref", []string{"run", "nope", "--dry-run"}, exitInvalidInput},
		{"unknown agent", []string{"run", "PROJ-1", "--dry-run", "--agent", "codex"}, exitInvalidInput},
		{"other project has no workspace", []string{"run", "OTH-1", "--dry-run"}, exitNotFound},
		{"several workspaces", nil, exitAmbiguous},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args == nil {
				mustRun(t, "workspace", "add", "myproject", "--path", t.TempDir())
				tt.args = []string{"run", "PROJ-1", "--dry-run"}
			}
			if r := run(t, "", tt.args...); r.code != tt.code || r.stdout != "" {
				t.Errorf("exit %d (want %d), stdout %q, stderr %q", r.code, tt.code, r.stdout, r.stderr)
			}
		})
	}
}

func TestRunWithoutDryRunLaunchesNothing(t *testing.T) {
	home := tempHome(t)
	r := run(t, "", "run", "PROJ-1")
	if r.code != exitFailure || !strings.Contains(r.stderr, "use --dry-run") {
		t.Errorf("exit %d, stderr %q", r.code, r.stderr)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("run without --dry-run touched storage: %v", err)
	}
}
