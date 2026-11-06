package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func seedCLIProjects(t *testing.T) {
	t.Helper()
	tempHome(t)
	mustRun(t, "project", "add", "myproject", "--prefix", "PROJ")
	mustRun(t, "project", "add", "other", "--prefix", "OTH")
}

func TestTaskAddShortcutAndCanonicalShareNumbering(t *testing.T) {
	seedCLIProjects(t)
	if out := mustRun(t, "add", "Fix export", "-p", "myproject"); out != "✓ Created PROJ-1: Fix export\n" {
		t.Errorf("add output = %q", out)
	}
	if out := mustRun(t, "task", "add", "Second", "--project", "myproject"); out != "✓ Created PROJ-2: Second\n" {
		t.Errorf("task add output = %q", out)
	}
	if out := mustRun(t, "add", "Elsewhere", "-p", "other"); out != "✓ Created OTH-1: Elsewhere\n" {
		t.Errorf("add in other project = %q", out)
	}
}

func TestTaskShowAndJSON(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Fix export", "-p", "myproject", "--description", "Details here",
		"--status", "in_progress", "--priority", "high")

	for _, args := range [][]string{{"show", "PROJ-1"}, {"task", "show", "proj-1"}} {
		out := mustRun(t, args...)
		for _, want := range []string{"PROJ-1  Fix export", "myproject", "in_progress", "high", "Details here"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v output missing %q:\n%s", args, want, out)
			}
		}
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(mustRun(t, "show", "PROJ-1", "--json")), &raw); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"ref": "PROJ-1", "title": "Fix export", "project": "myproject", "status": "in_progress",
		"priority": "high", "description": "Details here", "sequence_number": float64(1),
	}
	for k, v := range want {
		if raw[k] != v {
			t.Errorf("show --json %q = %v, want %v", k, raw[k], v)
		}
	}
	for _, k := range []string{"id", "project_id", "created_at", "updated_at"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("show --json missing %q", k)
		}
	}
}

func TestTaskAddDefaults(t *testing.T) {
	seedCLIProjects(t)
	var task map[string]any
	if err := json.Unmarshal([]byte(mustRun(t, "add", "x", "-p", "myproject", "--json")), &task); err != nil {
		t.Fatal(err)
	}
	if task["status"] != "todo" || task["priority"] != "none" || task["ref"] != "PROJ-1" {
		t.Errorf("defaults = %v", task)
	}
}

func TestTaskListFilters(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "a", "-p", "myproject")
	mustRun(t, "add", "b", "-p", "myproject", "--status", "done")
	mustRun(t, "add", "c", "-p", "other", "--status", "backlog")

	refs := func(args ...string) []string {
		t.Helper()
		var tasks []struct{ Ref string }
		if err := json.Unmarshal([]byte(mustRun(t, append(args, "--json")...)), &tasks); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, task := range tasks {
			out = append(out, task.Ref)
		}
		return out
	}
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"tasks"}, "PROJ-1 PROJ-2 OTH-1"},
		{[]string{"task", "list", "-p", "other"}, "OTH-1"},
		{[]string{"tasks", "--status", "todo"}, "PROJ-1"},
		{[]string{"tasks", "--status", "done,backlog"}, "PROJ-2 OTH-1"},
		{[]string{"tasks", "--status", "done", "--status", "todo", "-p", "myproject"}, "PROJ-1 PROJ-2"},
	}
	for _, tt := range tests {
		if got := strings.Join(refs(tt.args...), " "); got != tt.want {
			t.Errorf("%v = %q, want %q", tt.args, got, tt.want)
		}
	}

	out := mustRun(t, "tasks", "-p", "myproject")
	if !strings.Contains(out, "REF") || !strings.Contains(out, "PROJ-2") || !strings.Contains(out, "done") {
		t.Errorf("human list output = %q", out)
	}
}

func TestTaskEmptyList(t *testing.T) {
	seedCLIProjects(t)
	if out := mustRun(t, "tasks"); out != "No tasks across all projects.\n" {
		t.Errorf("empty list = %q", out)
	}
	if out := mustRun(t, "tasks", "--json"); strings.TrimSpace(out) != "[]" {
		t.Errorf("empty list --json = %q", out)
	}
}

func TestTaskExitCodes(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "a", "-p", "myproject")

	tests := []struct {
		name string
		args []string
		code int
	}{
		{"add without project", []string{"add", "x"}, exitInvalidInput},
		{"add without title", []string{"add", "-p", "myproject"}, exitInvalidInput},
		{"add empty title", []string{"add", "  ", "-p", "myproject"}, exitInvalidInput},
		{"add bad status", []string{"add", "x", "-p", "myproject", "--status", "open"}, exitInvalidInput},
		{"add bad priority", []string{"add", "x", "-p", "myproject", "--priority", "p1"}, exitInvalidInput},
		{"add unknown project", []string{"add", "x", "-p", "nope"}, exitNotFound},
		{"list bad status", []string{"tasks", "--status", "open"}, exitInvalidInput},
		{"list unknown project", []string{"tasks", "-p", "nope"}, exitNotFound},
		{"show malformed ref", []string{"show", "nope"}, exitInvalidInput},
		{"show unknown ref", []string{"show", "PROJ-99"}, exitNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, "", tt.args...)
			if r.code != tt.code {
				t.Errorf("exit = %d, want %d (stderr %q)", r.code, tt.code, r.stderr)
			}
			if !strings.HasPrefix(r.stderr, "taskship: ") || r.stdout != "" {
				t.Errorf("stdout %q, stderr %q; want error only on stderr", r.stdout, r.stderr)
			}
		})
	}
}

// Root-level task refs are not dispatch yet; they must not be treated as commands.
func TestUnknownCommandFails(t *testing.T) {
	tempHome(t)
	for _, args := range [][]string{{"taks"}, {"PROJ-1"}} {
		if r := run(t, "", args...); r.code == 0 {
			t.Errorf("%v succeeded; want failure", args)
		}
	}
}
