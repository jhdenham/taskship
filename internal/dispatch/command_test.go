package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskshipCommand(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Not on PATH: the running executable's path.
	t.Setenv("PATH", t.TempDir())
	if got := TaskshipCommand(); got != shellQuote(self) {
		t.Errorf("not on PATH = %q, want %q", got, shellQuote(self))
	}
	// A different taskship on PATH doesn't stand in for this one.
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "taskship"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", other)
	if got := TaskshipCommand(); got != shellQuote(self) {
		t.Errorf("other taskship on PATH = %q, want %q", got, shellQuote(self))
	}
	// This executable on PATH as taskship (here through a symlink): the name.
	dir := t.TempDir()
	if err := os.Symlink(self, filepath.Join(dir, "taskship")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := TaskshipCommand(); got != "taskship" {
		t.Errorf("on PATH = %q, want taskship", got)
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/Users/me/git/taskship/bin/taskship": "/Users/me/git/taskship/bin/taskship",
		"/Users/me/My Code/taskship":          "'/Users/me/My Code/taskship'",
		"/tmp/it's/taskship":                  `'/tmp/it'\''s/taskship'`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
