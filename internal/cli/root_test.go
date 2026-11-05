package cli

import (
	"bytes"
	"os"
	"testing"
)

func runRoot(t *testing.T, version string, args ...string) string {
	t.Helper()
	cmd := NewRootCmd(version)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return out.String()
}

func TestVersion(t *testing.T) {
	if got, want := runRoot(t, "v1.2.3", "--version"), "taskship version v1.2.3\n"; got != want {
		t.Errorf("--version = %q, want %q", got, want)
	}
	if got, want := runRoot(t, "", "--version"), "taskship version dev\n"; got != want {
		t.Errorf("--version without ldflags = %q, want %q", got, want)
	}
}

// --help and --version must not create user files.
func TestMetadataCommandsHaveNoSideEffects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, args := range [][]string{{"--version"}, {"--help"}} {
		runRoot(t, "", args...)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("HOME not empty after metadata commands: %v", entries)
	}
}
