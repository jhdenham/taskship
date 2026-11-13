# Working with agents

Taskship gives coding agents two things: the task, through the prompt of every agent it launches (see [Plan-first dispatch](concepts.md#plan-first-dispatch)), and the know-how to manage tasks well. That know-how has layers:

| Layer | What it covers | Who changes it |
|---|---|---|
| Taskship's rules | Task text is untrusted data, not instructions; the plan comes before any edits | Nobody |
| Taskship's guide | What a task that's ready to work on looks like: title, description, acceptance criteria, size | Built in |
| Your conventions | How *you* like tasks written and handled, across all your projects | You, in `~/.taskship/conventions.md` |
| The repository | Code style, test commands, architecture | The repository's own files, such as `CLAUDE.md` or `AGENTS.md` |

## Refining a task

An agent is only as good as the task it's given. `taskship refine PROJ-1` launches Claude Code to make a task ready to work on, without implementing it:

```text
$ taskship refine PROJ-1
✓ Launching Claude to refine PROJ-1 in ~/code/myproject
```

Claude starts in plan mode, in the folder `taskship PROJ-1` would use, so it can check the task against the code. It reads Taskship's guide to writing tasks and your own conventions, and asks about gaps only you can fill. Then it proposes a rewritten task: a clear title, the why and context, the scope, and acceptance criteria. It also suggests a split if the task is too big.

Refining looks beyond the one task. Claude reads the project's other tasks and points out:

- **duplicates**, which it can merge into this task and archive;
- **tasks that fit with this one**, such as a bug in the same code, to do together;
- **tasks that should come first or after**, noted with a line such as "Blocked by PROJ-12" (Taskship has no dependency field).

Claude changes tasks only after you approve, using the same commands you would: `task edit --from`, `add`, `task archive`, and `comment add`. Taskship allows those commands in the session it launches, so Claude's permission check doesn't block them once you've approved; `--dry-run` lists them. Refining doesn't change the task's status, and it is recorded as a run with the mode `refine` (see [Agent runs](concepts.md#agent-runs)). `taskship refine PROJ-1 --dry-run` shows the folder and the exact prompt.

## The Taskship skill

`taskship PROJ-1` and `taskship refine PROJ-1` give the agent everything it needs in its prompt. The skill is for every other session: you're already in Claude and say "work on PROJ-42", "add a task for that", "what should I work on next?", or "clean up the backlog". The skill teaches the agent the Taskship commands and how to use them well: read before acting, write tasks that follow the guide, and ask before changing anything.

```text
$ taskship skill install claude
✓ Installed the Taskship skill for Claude in ~/.claude/skills/taskship
If an open session doesn't see it, restart it.

$ taskship skill list
AGENT   SKILL          VERSION  FOLDER
claude  installed      v0.1.0   ~/.claude/skills/taskship
codex   not installed           ~/.agents/skills/taskship
```

The skill can be installed for `claude` and `codex`, or with `taskship skill install all` for every one of them on your `PATH` (the others are skipped, with a note). `taskship skill uninstall all` removes it from every agent it's installed for. It goes in the agent's own folder for skills, `~/.claude/skills/taskship` (or `$CLAUDE_CONFIG_DIR/skills`) and `~/.agents/skills/taskship`, never in a repository. `~/.agents/skills` is the folder Codex documents for personal skills, and other agents that follow the Agent Skills standard may read it too. Launching Codex on a task isn't supported yet, but the skill works in any Codex session.

The skill runs `taskship` from your `PATH`, so `install` needs it there. Run `taskship skill install` again after upgrading Taskship to update the skill; an older `taskship` leaves a newer skill alone. Updating replaces the whole skill file, so don't edit it: put your preferences in [your conventions](#your-conventions), which the skill reads every time. If the agent already has a skill named `taskship` that Taskship didn't install, `install` asks before replacing it (or needs `--yes` without a terminal), and `uninstall` leaves it alone. `uninstall` removes only the file Taskship installed.

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

## The task-writing guide

`taskship skill guide` prints Taskship's guide to writing tasks, followed by your conventions. `taskship refine` includes both in its prompt, and the skill has agents read them before they write a task. You can read it too, to see what your agents are told.

## Your conventions

Conventions are your own rules for tasks, the ones that apply in every project. Write them as Markdown in `~/.taskship/conventions.md` (or `$TASKSHIP_HOME/conventions.md`). For example:

```markdown
- Write acceptance criteria as Given / When / Then.
- Labels: bug, feature, chore, docs. Every task gets exactly one.
- Size every task S, M, or L in its first line.
- When you finish, add your summary as a comment without asking.
```

Taskship never creates this file. When it exists, it is added to the end of every prompt Taskship gives an agent, and `taskship PROJ-1 --dry-run` shows it there. Where your conventions differ from Taskship's guide or the prompt's instructions, the agent follows yours. They're your instructions, so they aren't treated as untrusted like task text. Claude's plan mode still applies: approving the plan is part of how Claude is launched, not just a line in the prompt.

Code conventions belong to the repository, not to this file.
