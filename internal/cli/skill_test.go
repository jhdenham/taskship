package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jhdenham/taskship/internal/guide"
)

func TestSkillGuide(t *testing.T) {
	home := tempHome(t)
	out := mustRun(t, "skill", "guide")
	if !strings.HasPrefix(out, guide.Tasks) || !strings.Contains(out, "None yet. The user can add their own in ") {
		t.Errorf("guide without conventions:\n%s", out)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("skill guide created %s", home)
	}

	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "conventions.md"), []byte("\n- Size every task S, M, or L.\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out = mustRun(t, "skill", "guide")
	if want := ". Where they differ from the guide above, follow them.\n\n- Size every task S, M, or L.\n"; !strings.HasSuffix(out, want) {
		t.Errorf("guide with conventions doesn't end with %q:\n%s", want, out)
	}
}

func TestSkillInstallListUninstall(t *testing.T) {
	tempHome(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	claude := filepath.Join(home, ".claude", "skills", "taskship")

	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if out := run(t, "v0.1.0", "skill", "install", "claude"); out.code != exitNotFound || !strings.Contains(out.stderr, "taskship isn't on your PATH") {
		t.Errorf("install without taskship on PATH: %+v", out)
	}
	if err := os.WriteFile(filepath.Join(bin, "taskship"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out := run(t, "v0.1.0", "skill", "install", "claude")
	if out.code != 0 || out.stdout != "✓ Installed the Taskship skill for Claude in ~/.claude/skills/taskship\nIf an open session doesn't see it, restart it.\n" {
		t.Fatalf("install: %+v", out)
	}
	b, err := os.ReadFile(filepath.Join(claude, "SKILL.md"))
	if err != nil || !strings.Contains(string(b), "<!-- Installed by taskship v0.1.0.") {
		t.Fatalf("installed skill: %v\n%s", err, b)
	}
	if out := run(t, "v0.1.0", "skill", "install", "claude"); out.stdout != "The Taskship skill for Claude is up to date.\n" {
		t.Errorf("reinstall: %+v", out)
	}
	if out := run(t, "v0.1.1", "skill", "install", "claude"); out.stdout != "✓ Updated the Taskship skill for Claude (v0.1.0 → v0.1.1)\n" {
		t.Errorf("upgrade: %+v", out)
	}
	if out := run(t, "v0.1.0", "skill", "install", "claude"); out.stdout != "The Taskship skill for Claude is from a newer taskship (v0.1.1), so it was left alone.\n" {
		t.Errorf("downgrade: %+v", out)
	}

	list := run(t, "v0.1.2", "skill", "list").stdout
	want := "AGENT   SKILL          VERSION  FOLDER\n" +
		"claude  outdated       v0.1.1   ~/.claude/skills/taskship\n" +
		"codex   not installed           ~/.agents/skills/taskship\n"
	if list != want {
		t.Errorf("list:\n%s\nwant:\n%s", list, want)
	}

	// A skill the user wrote is kept unless they say otherwise.
	codex := filepath.Join(home, ".agents", "skills", "taskship")
	os.MkdirAll(codex, 0o755)
	os.WriteFile(filepath.Join(codex, "SKILL.md"), []byte("---\nname: taskship\n---\nmine\n"), 0o644)
	if out := run(t, "v0.1.1", "skill", "install", "codex"); out.code != exitInvalidInput || !strings.Contains(out.stderr, "pass --yes to replace it") {
		t.Errorf("install over user skill: %+v", out)
	}
	if out := run(t, "v0.1.1", "skill", "uninstall", "codex"); out.code == 0 || !strings.Contains(out.stderr, "left alone") {
		t.Errorf("uninstall user skill: %+v", out)
	}
	if out := run(t, "v0.1.1", "skill", "install", "codex", "--yes"); out.code != 0 {
		t.Errorf("install --yes: %+v", out)
	}

	if out := run(t, "v0.1.1", "skill", "uninstall", "claude"); out.stdout != "✓ Removed the Taskship skill for Claude from ~/.claude/skills/taskship\n" {
		t.Errorf("uninstall: %+v", out)
	}
	if _, err := os.Stat(claude); !os.IsNotExist(err) {
		t.Errorf("skill folder left: %v", err)
	}
	if out := run(t, "v0.1.1", "skill", "uninstall", "claude"); out.code != 0 || out.stdout != "The Taskship skill isn't installed for Claude.\n" {
		t.Errorf("uninstall again: %+v", out)
	}
	if out := run(t, "", "skill", "install", "cursor"); out.code != exitInvalidInput {
		t.Errorf("unknown agent: %+v", out)
	}
}

func TestSkillInstallUninstallAll(t *testing.T) {
	tempHome(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := os.WriteFile(filepath.Join(bin, "taskship"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if out := run(t, "v1", "skill", "install", "all"); out.code != exitNotFound ||
		out.stderr != "taskship: No supported agents found on your PATH (claude, codex). Install one, or name it: \"taskship skill install claude\"\n" {
		t.Errorf("install all, no agents: %+v", out)
	}

	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "Skipped Codex: codex isn't on your PATH.\n" +
		"✓ Installed the Taskship skill for Claude in ~/.claude/skills/taskship\n" +
		"If an open session doesn't see it, restart it.\n"
	if out := run(t, "v1", "skill", "install", "all"); out.code != 0 || out.stdout != want {
		t.Errorf("install all:\n%s\nwant:\n%s", out.stdout, want)
	}
	if out := run(t, "v1", "skill", "install", "all"); out.stdout != "Skipped Codex: codex isn't on your PATH.\nThe Taskship skill for Claude is up to date.\n" {
		t.Errorf("install all again: %+v", out)
	}

	// A skill the user wrote for Codex is left alone; Claude's is removed.
	codex := filepath.Join(home, ".agents", "skills", "taskship")
	os.MkdirAll(codex, 0o755)
	os.WriteFile(filepath.Join(codex, "SKILL.md"), []byte("mine\n"), 0o644)
	want = "✓ Removed the Taskship skill for Claude from ~/.claude/skills/taskship\n" +
		"Left ~/.agents/skills/taskship alone: Taskship didn't install the skill there.\n"
	if out := run(t, "v1", "skill", "uninstall", "all"); out.code != 0 || out.stdout != want {
		t.Errorf("uninstall all:\n%s\nwant:\n%s", out.stdout, want)
	}
	os.RemoveAll(codex)
	if out := run(t, "v1", "skill", "uninstall", "all"); out.stdout != "The Taskship skill isn't installed for any agent.\n" {
		t.Errorf("uninstall all, none: %+v", out)
	}
}
