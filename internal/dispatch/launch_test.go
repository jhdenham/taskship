package dispatch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestClaudeArgs(t *testing.T) {
	prompt := "-x $(touch pwned); `id` && echo 'hi' \"there\""
	args, err := ClaudeArgs("sess", prompt)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--permission-mode", "plan", "--session-id", "sess", "--", prompt}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
	if _, err := ClaudeArgs("sess", strings.Repeat("x", maxPromptBytes)); err != nil {
		t.Errorf("prompt at the limit: %v", err)
	}
	if _, err := ClaudeArgs("sess", strings.Repeat("x", maxPromptBytes+1)); err == nil || !strings.Contains(err.Error(), "128 KB") {
		t.Errorf("prompt over the limit: err = %v", err)
	}
}

// fakeAgent writes a script that records its folder and arguments (one per
// NUL-terminated entry) in out, then runs body.
func fakeAgent(t *testing.T, body string) (path, out string) {
	t.Helper()
	dir := t.TempDir()
	path, out = filepath.Join(dir, "agent"), filepath.Join(dir, "out")
	script := "#!/bin/sh\npwd > '" + out + "'\nprintf '%s\\0' \"$@\" >> '" + out + "'\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, out
}

func TestLaunch(t *testing.T) {
	path, out := fakeAgent(t, "exit 3")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--", "a b; $(touch pwned)", "it's \"quoted\"\nnext line"}
	started := false
	code, err := Launch(path, args, nil, dir, func() { started = true })
	if err != nil || code != 3 || !started {
		t.Fatalf("Launch = %d, %v (started %v), want 3", code, err, started)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	cwd, rest, _ := strings.Cut(string(data), "\n")
	if cwd != dir {
		t.Errorf("agent ran in %q, want %q", cwd, dir)
	}
	if got := strings.Split(strings.TrimSuffix(rest, "\x00"), "\x00"); !reflect.DeepEqual(got, args) {
		t.Errorf("agent got %q, want %q", got, args)
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Error("task text ran as a shell command")
	}
}

func TestLaunchExitCodes(t *testing.T) {
	for _, c := range []struct {
		body string
		want int
	}{
		{"exit 0", 0},
		{"kill -TERM $$", 128 + 15},
	} {
		path, _ := fakeAgent(t, c.body)
		if code, err := Launch(path, nil, nil, t.TempDir(), func() {}); err != nil || code != c.want {
			t.Errorf("%s: Launch = %d, %v, want %d", c.body, code, err, c.want)
		}
	}
}

func TestLaunchNotStarted(t *testing.T) {
	called := false
	if _, err := Launch(filepath.Join(t.TempDir(), "missing"), nil, nil, t.TempDir(), func() { called = true }); err == nil || called {
		t.Errorf("missing executable: err = %v, started called = %v", err, called)
	}
}
