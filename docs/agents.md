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

Claude starts in plan mode, in the folder `taskship PROJ-1` would use, so it can check the task against the code. It reads Taskship's guide to writing tasks and your conventions, and asks about gaps only you can fill. Then it proposes a rewritten task: a clear title, the why and context, the scope, and acceptance criteria. It also suggests a split if the task is too big.

Refining looks beyond the one task. Claude reads the project's other tasks and points out:

- **duplicates**, which it can merge into this task and archive;
- **tasks that fit with this one**, such as a bug in the same code, to do together;
- **tasks that should come first or after**, noted with a line such as "Blocked by PROJ-12" (Taskship has no dependency field).

Claude changes tasks only after you approve, using the same commands you would: `task edit --from`, `add`, `task archive`, and `comment add`. Refining doesn't change the task's status, and it is recorded as a run with the mode `refine` (see [Agent runs](concepts.md#agent-runs)). `taskship refine PROJ-1 --dry-run` shows the folder and the exact prompt.

## The task-writing guide

`taskship skill guide` prints Taskship's guide to writing tasks, followed by your conventions. `taskship refine` includes both in its prompt, and agents read them before they write a task. You can read it too, to see what your agents are told.

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
