package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

type result struct {
	stdout, stderr string
	code           int
}

func run(t *testing.T, version string, args ...string) result {
	t.Helper()
	cmd := NewRootCmd(version)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	code := Execute(cmd)
	return result{out.String(), errb.String(), code}
}

// mustRun runs a command that is expected to succeed.
func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	r := run(t, "", args...)
	if r.code != 0 {
		t.Fatalf("%v: exit %d, stderr %q", args, r.code, r.stderr)
	}
	return r.stdout
}

// tempHome points TASKSHIP_HOME at a fresh, not-yet-created directory.
func tempHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "taskship-home")
	t.Setenv("TASKSHIP_HOME", home)
	return home
}

func TestVersion(t *testing.T) {
	if got, want := run(t, "v1.2.3", "--version").stdout, "taskship version v1.2.3\n"; got != want {
		t.Errorf("--version = %q, want %q", got, want)
	}
	if got, want := run(t, "", "--version").stdout, "taskship version dev\n"; got != want {
		t.Errorf("--version without ldflags = %q, want %q", got, want)
	}
}

// --help and --version must not create user files.
func TestMetadataCommandsHaveNoSideEffects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TASKSHIP_HOME", "")
	for _, args := range [][]string{{"--version"}, {"--help"}, {"project", "--help"}, {"project", "add", "--help"}, {"task", "--help"}, {"add", "--help"}, {"tasks", "--help"}, {"run", "--help"}} {
		if r := run(t, "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d", args, r.code)
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("HOME not empty after metadata commands: %v", entries)
	}
}

func TestFirstStatefulCommandInitializesStorage(t *testing.T) {
	home := tempHome(t)
	if out := mustRun(t, "project", "list"); !strings.Contains(out, "No projects") {
		t.Errorf("project list on empty store = %q", out)
	}
	info, err := os.Stat(home)
	if err != nil {
		t.Fatalf("home not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("home permissions = %o, want 700", perm)
	}
	if _, err := os.Stat(filepath.Join(home, store.DBFile)); err != nil {
		t.Errorf("database not created: %v", err)
	}
}

func TestProjectCommands(t *testing.T) {
	tempHome(t)

	if out := mustRun(t, "project", "add", "myproject", "--prefix", "PROJ"); out != "✓ Created project myproject (PROJ)\n" {
		t.Errorf("add output = %q", out)
	}
	mustRun(t, "project", "add", "another", "--prefix", "ANO")

	list := mustRun(t, "project", "list")
	if !strings.HasPrefix(list, "NAME       PREFIX\n") || !strings.Contains(list, "another") ||
		strings.Index(list, "another") > strings.Index(list, "myproject") {
		t.Errorf("list output = %q", list)
	}

	var projects []core.Project
	if err := json.Unmarshal([]byte(mustRun(t, "project", "list", "--json")), &projects); err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Errorf("list --json returned %d projects", len(projects))
	}

	show := mustRun(t, "project", "show", "myproject")
	if !strings.HasPrefix(show, "Name:     myproject\n") || !strings.Contains(show, "PROJ") {
		t.Errorf("show output = %q", show)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(mustRun(t, "project", "show", "myproject", "--json")), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "name", "task_prefix", "description", "created_at", "updated_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("show --json missing %q: %v", key, raw)
		}
	}
	if _, ok := raw["slug"]; ok {
		t.Errorf("show --json includes slug: %v", raw)
	}
	if _, ok := raw["archived_at"]; ok {
		t.Errorf("show --json includes archived_at for an unarchived project")
	}
}

func TestProjectEmptyListJSONIsArray(t *testing.T) {
	tempHome(t)
	if out := mustRun(t, "project", "list", "--json"); strings.TrimSpace(out) != "[]" {
		t.Errorf("empty list --json = %q, want []", out)
	}
}

func TestExitCodes(t *testing.T) {
	tempHome(t)
	mustRun(t, "project", "add", "myproject", "--prefix", "PROJ")

	tests := []struct {
		name string
		args []string
		code int
	}{
		{"missing prefix", []string{"project", "add", "x"}, exitInvalidInput},
		{"missing name", []string{"project", "add", "--prefix", "X"}, exitInvalidInput},
		{"unknown flag", []string{"project", "list", "--bogus"}, exitInvalidInput},
		{"invalid name", []string{"project", "add", "Bad", "--prefix", "BAD"}, exitInvalidInput},
		{"duplicate name", []string{"project", "add", "myproject", "--prefix", "OTHER"}, exitInvalidInput},
		{"duplicate prefix", []string{"project", "add", "other", "--prefix", "PROJ"}, exitInvalidInput},
		{"unknown project", []string{"project", "show", "nope"}, exitNotFound},
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
