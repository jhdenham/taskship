package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const editedTask = `---
ref: PROJ-1
project: myproject
status: in_progress
priority: high
---
# Edited title

Body.

## Acceptance criteria

- [ ] One
`

func TestShowMarkdown(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Title", "-p", "myproject", "--description", "Body.")
	want := "---\nref: PROJ-1\nproject: myproject\nstatus: todo\npriority: none\n---\n# Title\n\nBody.\n"
	if out := mustRun(t, "show", "PROJ-1", "--md"); out != want {
		t.Errorf("show --md =\n%s", out)
	}
	if r := run(t, "", "show", "PROJ-1", "--md", "--json"); r.code != exitInvalidInput {
		t.Errorf("--md --json: exit %d", r.code)
	}
}

// fakeEditor sets $VISUAL to a script that replaces the file it is given
// with content, and points temporary files at a fresh directory.
func fakeEditor(t *testing.T, content string) (tmp string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "content.md")
	script := filepath.Join(dir, "editor.sh")
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("cat '"+src+"' > \"$1\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "sh "+script) // an editor command with arguments
	tmp = t.TempDir()
	t.Setenv("TMPDIR", tmp)
	return tmp
}

func assertNoDrafts(t *testing.T, tmp string) {
	t.Helper()
	if entries, _ := os.ReadDir(tmp); len(entries) != 0 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

func TestEditInEditor(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Title", "-p", "myproject", "--description", "Old body")
	tmp := fakeEditor(t, editedTask)

	if out := mustRun(t, "task", "edit", "PROJ-1"); out != "✓ Updated PROJ-1: Edited title\n" {
		t.Errorf("edit output = %q", out)
	}
	task := taskJSON(t, "PROJ-1")
	want := map[string]any{"title": "Edited title", "status": "in_progress", "priority": "high",
		"description": "Body.\n\n## Acceptance criteria\n\n- [ ] One"}
	for k, v := range want {
		if task[k] != v {
			t.Errorf("%s = %q, want %q", k, task[k], v)
		}
	}
	assertNoDrafts(t, tmp)

	// Saving the same content again changes nothing.
	updated := task["updated_at"]
	if out := mustRun(t, "task", "edit", "PROJ-1"); out != "No changes to PROJ-1\n" {
		t.Errorf("unchanged edit output = %q", out)
	}
	if taskJSON(t, "PROJ-1")["updated_at"] != updated {
		t.Error("unchanged edit touched updated_at")
	}
	assertNoDrafts(t, tmp)
}

func TestEditInEditorKeepsInvalidDraft(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Title", "-p", "myproject")
	tmp := fakeEditor(t, strings.Replace(editedTask, "status: in_progress", "status: open", 1))

	r := run(t, "", "task", "edit", "PROJ-1")
	if r.code != exitInvalidInput || !strings.Contains(r.stderr, "unknown status") || !strings.Contains(r.stderr, "--from "+tmp) {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	entries, _ := os.ReadDir(tmp)
	if len(entries) != 1 {
		t.Fatalf("drafts = %v, want one", entries)
	}
	if task := taskJSON(t, "PROJ-1"); task["title"] != "Title" {
		t.Errorf("invalid edit changed the task: %v", task)
	}

	// Fixing the draft and passing it with --from applies it and removes nothing else.
	draft := filepath.Join(tmp, entries[0].Name())
	os.WriteFile(draft, []byte(editedTask), 0o644)
	mustRun(t, "task", "edit", "PROJ-1", "--from", draft)
	if task := taskJSON(t, "PROJ-1"); task["status"] != "in_progress" {
		t.Errorf("status after --from = %v", task["status"])
	}
}

func TestEditorFailureLeavesTaskUnchanged(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Title", "-p", "myproject")
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("VISUAL", "false")

	if r := run(t, "", "task", "edit", "PROJ-1"); r.code != exitFailure || !strings.Contains(r.stderr, "editor false") {
		t.Errorf("exit %d, stderr %q", r.code, r.stderr)
	}
	assertNoDrafts(t, tmp)
}

func TestEditFromStdin(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Title", "-p", "myproject", "--description", "Keep me")

	// A document without front matter changes only the title and description.
	cmd := NewRootCmd("")
	var out, errb bytes.Buffer
	cmd.SetIn(strings.NewReader("# New title\n\nKeep me\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"task", "edit", "proj-1", "--from", "-"})
	if code := Execute(cmd); code != exitOK || out.String() != "✓ Updated PROJ-1: New title\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
	task := taskJSON(t, "PROJ-1")
	if task["title"] != "New title" || task["description"] != "Keep me" || task["status"] != "todo" {
		t.Errorf("task = %v", task)
	}
}

func TestEditFromErrors(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Title", "-p", "myproject")
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte(content), 0o644)
		return path
	}

	tests := []struct {
		name string
		args []string
		code int
	}{
		{"with field flags", []string{"--from", write("ok.md", editedTask), "--title", "x"}, exitInvalidInput},
		{"other ref", []string{"--from", write("ref.md", strings.Replace(editedTask, "PROJ-1", "PROJ-2", 1))}, exitInvalidInput},
		{"other project", []string{"--from", write("proj.md", strings.Replace(editedTask, "myproject", "other", 1))}, exitInvalidInput},
		{"no heading", []string{"--from", write("bad.md", "just text\n")}, exitInvalidInput},
		{"empty title", []string{"--from", write("empty.md", "# \n")}, exitInvalidInput},
		{"missing file", []string{"--from", filepath.Join(dir, "nope.md")}, exitFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, "", append([]string{"task", "edit", "PROJ-1"}, tt.args...)...)
			if r.code != tt.code || r.stdout != "" {
				t.Errorf("exit = %d, want %d (stdout %q, stderr %q)", r.code, tt.code, r.stdout, r.stderr)
			}
		})
	}
	if task := taskJSON(t, "PROJ-1"); task["title"] != "Title" {
		t.Errorf("failed edits changed the task: %v", task)
	}
}

func TestExport(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "a", "-p", "myproject", "--description", "Body")
	mustRun(t, "add", "b", "-p", "myproject")
	mustRun(t, "add", "c", "-p", "other")
	mustRun(t, "task", "archive", "PROJ-2")
	dir := filepath.Join(t.TempDir(), "export")

	if out := mustRun(t, "export", dir, "-p", "myproject"); out != "✓ Exported 1 task to "+dir+"\n" {
		t.Errorf("export output = %q", out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "PROJ-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := mustRun(t, "show", "PROJ-1", "--md"); string(got) != want {
		t.Errorf("PROJ-1.md =\n%s\nwant\n%s", got, want)
	}

	if out := mustRun(t, "export", dir); out != "✓ Exported 2 tasks to "+dir+"\n" {
		t.Errorf("export all output = %q", out)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "OTH-1.md,PROJ-1.md" {
		t.Errorf("exported files = %v", names)
	}
	if r := run(t, "", "export", dir, "-p", "nope"); r.code != exitNotFound {
		t.Errorf("unknown project: exit %d", r.code)
	}
}
