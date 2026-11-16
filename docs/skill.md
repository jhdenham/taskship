# Skill

`taskship PROJ-1` and `taskship refine PROJ-1` give the agent everything it needs in its prompt. The skill is for every other session: you're already talking to Claude Code or Codex and say "work on PROJ-42", "add a task for that", "what should I work on next?", or "clean up the backlog". The skill teaches the agent the Taskship commands and how to use them well: read before acting, write tasks that follow the [guide](conventions.md#the-task-writing-guide), and ask before changing anything.

The skill is optional: launching an agent works without it.

```text
$ taskship skill install claude
✓ Installed the Taskship skill for Claude in ~/.claude/skills/taskship
If an open session doesn't see it, restart it.

$ taskship skill list
AGENT   SKILL          VERSION  FOLDER
claude  installed      v0.1.0   ~/.claude/skills/taskship
codex   not installed           ~/.agents/skills/taskship
```

## Installing

The skill can be installed for `claude` and `codex`, or with `taskship skill install all` for every one of them on your `PATH` (the others are skipped, with a note). `taskship skill uninstall all` removes it from every agent it's installed for. It goes in the agent's own folder for skills, `~/.claude/skills/taskship` (or `$CLAUDE_CONFIG_DIR/skills`) and `~/.agents/skills/taskship`, never in a repository. `~/.agents/skills` is the folder Codex documents for personal skills, and other agents that follow the Agent Skills standard may read it too. Launching Codex on a task isn't supported yet, but the skill works in any Codex session.

The skill runs `taskship` from your `PATH`, so `install` needs it there. [`taskship upgrade`](quickstart.md#upgrading) updates the skill for each agent where it's installed. After upgrading another way, such as the install script or `go install`, run `taskship skill install` again to update it; an older `taskship` leaves a newer skill alone. Updating replaces the whole skill file, so don't edit it: put your preferences in [your conventions](conventions.md#your-conventions), which the skill reads every time. If the agent already has a skill named `taskship` that Taskship didn't install, `install` asks before replacing it (or needs `--yes` without a terminal), and `uninstall` leaves it alone. `uninstall` removes only the file Taskship installed.

## Permissions in Claude Code

In a session Taskship didn't launch, Claude's permission settings apply to `taskship` commands like any other, so Claude may ask before running a command that changes a task. To skip those prompts, add rules to `~/.claude/settings.json`, for example:

```json
{
  "permissions": {
    "allow": [
      "Bash(taskship show *)",
      "Bash(taskship tasks *)",
      "Bash(taskship skill guide *)",
      "Bash(taskship add *)",
      "Bash(taskship comment add *)",
      "Bash(taskship note add *)",
      "Bash(taskship task edit *)"
    ]
  }
}
```

Taskship never changes your Claude settings itself.

## Agent sandboxes

An agent's sandbox can stop `taskship` from writing to `~/.taskship` (or `$TASKSHIP_HOME`, if you set it). Reading tasks still works, but commands that change them fail with "can't write to ~/.taskship". To fix it, allow the agent to write to that folder.

Codex's sandbox is on by default. Add the folder as a writable root in `~/.codex/config.toml`:

```toml
sandbox_mode = "workspace-write"

[sandbox_workspace_write]
writable_roots = ["~/.taskship"]
```

Or allow it for a single session with `codex --add-dir ~/.taskship`. Commands that stay inside the sandbox run without asking, so Codex needs no allow list for `taskship` commands.

Claude Code's sandbox is off by default. If you have it turned on, add the folder to `~/.claude/settings.json`:

```json
{
  "sandbox": {
    "filesystem": {
      "allowWrite": ["~/.taskship"]
    }
  }
}
```

Taskship never changes your agent settings itself.
