package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func taskJSON(t *testing.T, ref string) map[string]any {
	t.Helper()
	var task map[string]any
	if err := json.Unmarshal([]byte(mustRun(t, "show", ref, "--json")), &task); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestTaskEdit(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Old title", "-p", "myproject", "--description", "keep me")

	if out := mustRun(t, "task", "edit", "proj-1", "--title", "New title", "--status", "in_progress", "--priority", "high"); out != "Updated PROJ-1: New title\n" {
		t.Errorf("edit output = %q", out)
	}
	task := taskJSON(t, "PROJ-1")
	for k, want := range map[string]any{"title": "New title", "status": "in_progress", "priority": "high", "description": "keep me"} {
		if task[k] != want {
			t.Errorf("%s = %v, want %v", k, task[k], want)
		}
	}

	// An explicitly empty description clears it.
	mustRun(t, "task", "edit", "PROJ-1", "--description", "")
	if task := taskJSON(t, "PROJ-1"); task["description"] != "" {
		t.Errorf("description after clearing = %q", task["description"])
	}
}

func TestTaskArchiveRestore(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "a", "-p", "myproject")
	mustRun(t, "add", "b", "-p", "myproject")

	if out := mustRun(t, "task", "archive", "PROJ-1"); out != "Archived PROJ-1: a\n" {
		t.Errorf("archive output = %q", out)
	}
	mustRun(t, "task", "archive", "PROJ-1") // repeating is fine

	if out := mustRun(t, "tasks"); strings.Contains(out, "PROJ-1") {
		t.Errorf("archived task in default list:\n%s", out)
	}
	if out := mustRun(t, "tasks", "--archived"); !strings.Contains(out, "a (archived)") || !strings.Contains(out, "PROJ-2") {
		t.Errorf("tasks --archived:\n%s", out)
	}
	if out := mustRun(t, "show", "PROJ-1"); !strings.Contains(out, "Archived:") {
		t.Errorf("show of archived task lacks Archived line:\n%s", out)
	}
	if _, ok := taskJSON(t, "PROJ-1")["archived_at"]; !ok {
		t.Error("show --json of archived task lacks archived_at")
	}

	if out := mustRun(t, "task", "restore", "PROJ-1"); out != "Restored PROJ-1: a\n" {
		t.Errorf("restore output = %q", out)
	}
	if out := mustRun(t, "tasks"); !strings.Contains(out, "PROJ-1") {
		t.Errorf("restored task missing from list:\n%s", out)
	}
}

func TestProjectEdit(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "a", "-p", "myproject")

	if out := mustRun(t, "project", "edit", "myproject", "--name", "renamed", "--description", "Renamed"); out != "Updated project renamed\n" {
		t.Errorf("edit output = %q", out)
	}
	if out := mustRun(t, "project", "show", "renamed"); !strings.Contains(out, "Renamed") || !strings.Contains(out, "PROJ") {
		t.Errorf("show after rename:\n%s", out)
	}
	if task := taskJSON(t, "PROJ-1"); task["project"] != "renamed" {
		t.Errorf("task project after rename = %v", task["project"])
	}
	mustRun(t, "add", "b", "-p", "renamed")
}

func TestProjectArchiveRestore(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "a", "-p", "myproject")
	mustRun(t, "add", "b", "-p", "other")

	if out := mustRun(t, "project", "archive", "myproject"); out != "Archived project myproject\n" {
		t.Errorf("archive output = %q", out)
	}
	if out := mustRun(t, "project", "list"); strings.Contains(out, "myproject") {
		t.Errorf("archived project in default list:\n%s", out)
	}
	if out := mustRun(t, "project", "list", "--archived"); !strings.Contains(out, "(archived)") {
		t.Errorf("project list --archived:\n%s", out)
	}
	if out := mustRun(t, "tasks"); strings.Contains(out, "PROJ-1") || !strings.Contains(out, "OTH-1") {
		t.Errorf("tasks with archived project:\n%s", out)
	}
	if r := run(t, "", "add", "c", "-p", "myproject"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "project restore myproject") {
		t.Errorf("add to archived project: exit %d, stderr %q", r.code, r.stderr)
	}

	if out := mustRun(t, "project", "restore", "myproject"); out != "Restored project myproject\n" {
		t.Errorf("restore output = %q", out)
	}
	if out := mustRun(t, "tasks"); !strings.Contains(out, "PROJ-1") {
		t.Errorf("tasks after restore:\n%s", out)
	}
}

func TestEditArchiveExitCodes(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "a", "-p", "myproject")

	tests := []struct {
		name string
		args []string
		code int
	}{
		{"task edit empty title", []string{"task", "edit", "PROJ-1", "--title", ""}, exitInvalidInput},
		{"task edit bad status", []string{"task", "edit", "PROJ-1", "--status", "open"}, exitInvalidInput},
		{"task edit bad priority", []string{"task", "edit", "PROJ-1", "--priority", "p1"}, exitInvalidInput},
		{"task edit unknown", []string{"task", "edit", "PROJ-9", "--title", "x"}, exitNotFound},
		{"task archive unknown", []string{"task", "archive", "PROJ-9"}, exitNotFound},
		{"task restore malformed", []string{"task", "restore", "nope"}, exitInvalidInput},
		{"project edit nothing", []string{"project", "edit", "myproject"}, exitInvalidInput},
		{"project edit name taken", []string{"project", "edit", "myproject", "--name", "other"}, exitInvalidInput},
		{"project edit prefix", []string{"project", "edit", "myproject", "--prefix", "NEW"}, exitInvalidInput},
		{"project edit unknown", []string{"project", "edit", "nope", "--name", "x"}, exitNotFound},
		{"project archive unknown", []string{"project", "archive", "nope"}, exitNotFound},
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
