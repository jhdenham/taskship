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

The skill runs `taskship` from your `PATH`, so `install` needs it there. Run `taskship skill install` again after upgrading Taskship to update the skill; an older `taskship` leaves a newer skill alone. Updating replaces the whole skill file, so don't edit it: put your preferences in [your conventions](conventions.md#your-conventions), which the skill reads every time. If the agent already has a skill named `taskship` that Taskship didn't install, `install` asks before replacing it (or needs `--yes` without a terminal), and `uninstall` leaves it alone. `uninstall` removes only the file Taskship installed.

## Permissions in Claude Code

In a session Taskship didn't launch, Claude's permission settings apply to `taskship` commands like any other, so auto mode may stop to check a command that changes a task. To allow them, add rules to `~/.claude/settings.json`, for example:

```json
{
  "permissions": {
    "allow": [
      "Bash(taskship show *)",
      "Bash(taskship tasks *)",
      "Bash(taskship skill guide *)",
      "Bash(taskship add *)",
      "Bash(taskship comment add *)",
      "Bash(taskship task edit *)"
    ]
  }
}
```

Taskship never changes your Claude settings itself.
