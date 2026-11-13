package dispatch

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jhdenham/taskship/internal/core"
)

// A run may change only its own task; refining may change any task.
func TestAllowedCommands(t *testing.T) {
	reads := []string{"Bash(taskship show *)", "Bash(taskship tasks *)", "Bash(taskship runs *)",
		"Bash(taskship comment list *)", "Bash(taskship skill guide *)", "Bash(taskship add *)"}
	run := append(append([]string{}, reads...), "Bash(taskship comment add PROJ-42 *)", "Bash(taskship task edit PROJ-42 --status *)")
	if got := AllowedCommands(core.RunModeRun, "PROJ-42", "taskship"); !reflect.DeepEqual(got, run) {
		t.Errorf("run = %q", got)
	}
	refine := append(append([]string{}, reads...), "Bash(taskship task edit *)", "Bash(taskship comment add *)", "Bash(taskship task archive *)")
	if got := AllowedCommands(core.RunModeRefine, "PROJ-42", "taskship"); !reflect.DeepEqual(got, refine) {
		t.Errorf("refine = %q", got)
	}
}

// Every Taskship command a prompt tells the agent to run is allowed, so a
// new command in a prompt can't be blocked once the user approves the plan.
func TestPromptCommandsAreAllowed(t *testing.T) {
	task := core.Task{Ref: "PROJ-42", Project: "myproject", Title: "Fix the export"}
	cmd := regexp.MustCompile("T ([^\n`]+)")
	for mode, build := range map[core.RunMode]func(core.Task, []core.Comment, string, string) string{
		core.RunModeRun: BuildPrompt, core.RunModeRefine: BuildRefinePrompt,
	} {
		var prefixes []string
		for _, rule := range AllowedCommands(mode, task.Ref, "T") {
			prefixes = append(prefixes, strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), "*)"))
		}
		matches := cmd.FindAllStringSubmatch(build(task, nil, "T", ""), -1)
		if len(matches) < 4 {
			t.Fatalf("%s: found only %d commands in the prompt", mode, len(matches))
		}
		for _, m := range matches {
			if m[1] == "TASK_REF" { // the user launches that one
				continue
			}
			ok := false
			for _, p := range prefixes {
				ok = ok || strings.HasPrefix("T "+m[1]+" ", p)
			}
			if !ok {
				t.Errorf("%s: the prompt names %q, which isn't allowed", mode, "T "+m[1])
			}
		}
	}
}

func TestClaudeArgs(t *testing.T) {
	prompt := "-x $(touch pwned); `id` && echo 'hi' \"there\""
	args, err := ClaudeArgs("sess", prompt, []string{"Bash(taskship show *)", "Bash(taskship tasks *)"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--permission-mode", "plan", "--session-id", "sess", "--allowedTools", "Bash(taskship show *)", "Bash(taskship tasks *)", "--", prompt}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args = %q, want %q", args, want)
	}
	if args, _ := ClaudeArgs("sess", prompt, nil); !reflect.DeepEqual(args, []string{"--permission-mode", "plan", "--session-id", "sess", "--", prompt}) {
		t.Errorf("args without allowed commands = %q", args)
	}
	if _, err := ClaudeArgs("sess", strings.Repeat("x", maxPromptBytes), nil); err != nil {
		t.Errorf("prompt at the limit: %v", err)
	}
	if _, err := ClaudeArgs("sess", strings.Repeat("x", maxPromptBytes+1), nil); err == nil || !strings.Contains(err.Error(), "128 KB") || !strings.Contains(err.Error(), "its comments, or your conventions") {
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

// A terminate request to taskship reaches the agent, and taskship survives
// it to report how the agent exited.
func TestLaunchPassesTerminateToAgent(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	path, _ := fakeAgent(t, "trap 'exit 7' TERM\ntouch '"+ready+"'\nwhile :; do sleep 0.05; done")
	code, err := Launch(path, nil, nil, t.TempDir(), func() {
		go func() {
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
				if _, err := os.Stat(ready); err == nil {
					break
				}
			}
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
		}()
	})
	if err != nil || code != 7 {
		t.Errorf("Launch = %d, %v, want 7", code, err)
	}
}
