package skill

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	s := Render("v0.1.0")
	if strings.Contains(s, "{{") || !strings.HasPrefix(s, "---\nname: taskship\ndescription: ") ||
		!strings.Contains(s, marker+"v0.1.0. ") || !strings.Contains(s, "`taskship show REF --json`") {
		t.Errorf("Render:\n%s", s)
	}
}

func TestDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/c")
	t.Setenv("CODEX_HOME", "/x")
	t.Setenv("HOME", "/home/me")
	for agent, want := range map[string]string{"claude": "/c/skills/taskship", "codex": "/home/me/.agents/skills/taskship"} {
		if got, err := Dir(agent); err != nil || got != want {
			t.Errorf("Dir(%s) = %q, %v; want %q", agent, got, err, want)
		}
	}
	if _, err := Dir("cursor"); err == nil {
		t.Error("Dir(cursor) succeeded")
	}
}

func TestInstallLifecycle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", "taskship")
	v1, v2 := Render("v0.1.0"), Render("v0.2.0")
	state := func(want string) State {
		t.Helper()
		s, _, err := Status(dir, want)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	if s := state(v1); s != NotInstalled {
		t.Errorf("before install: %s", s)
	}
	if err := Install(dir, v1, NotInstalled); err != nil {
		t.Fatal(err)
	}
	if s, v, _ := Status(dir, v1); s != Installed || v != "v0.1.0" {
		t.Errorf("after install: %s %s", s, v)
	}
	if s := state(v2); s != Outdated {
		t.Errorf("older version: %s", s)
	}
	if s := state(Render("dev")); s != Outdated {
		t.Errorf("dev build: %s", s)
	}
	os.WriteFile(filepath.Join(dir, File), []byte(v1+"\nmy note\n"), 0o644)
	if s := state(v1); s != Modified {
		t.Errorf("edited: %s", s)
	}
	if err := Install(dir, v2, Modified); err != nil || state(v2) != Installed {
		t.Errorf("upgrade: %v, %s", err, state(v2))
	}
	if s := state(v1); s != Newer {
		t.Errorf("seen by an older taskship: %s", s)
	}

	// Uninstall leaves files Taskship didn't write, and so the folder.
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("mine"), 0o644)
	if removed, err := Uninstall(dir); err != nil || !removed {
		t.Fatal(removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); err != nil {
		t.Errorf("uninstall removed the user's file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, File)); !os.IsNotExist(err) {
		t.Errorf("uninstall left %s: %v", File, err)
	}

	// A folder without Taskship's skill isn't ours: uninstall leaves it, and
	// install replaces it whole.
	if s := state(v1); s != NotOurs {
		t.Errorf("user folder: %s", s)
	}
	if _, err := Uninstall(dir); !errors.Is(err, ErrNotOurs) {
		t.Errorf("uninstall user folder: %v", err)
	}
	if err := Install(dir, v1, NotOurs); err != nil || state(v1) != Installed {
		t.Errorf("replace: %v, %s", err, state(v1))
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); !os.IsNotExist(err) {
		t.Errorf("replace kept the old folder's files: %v", err)
	}

	// Uninstalling the last file removes the folder; uninstalling again is fine.
	if removed, err := Uninstall(dir); err != nil || !removed {
		t.Fatal(removed, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("folder left behind: %v", err)
	}
	if removed, err := Uninstall(dir); err != nil || removed {
		t.Errorf("uninstall when missing: %v, %v", removed, err)
	}
}
