# Launching agents

Taskship launches a coding agent on a task in your terminal, for one of two jobs:

1. **[Refine](#refine):** `taskship refine PROJ-1` launches an agent to turn a rough idea into a task that's ready to work on.
2. **[Run](#run):** `taskship PROJ-1` launches an agent to plan the task, wait for your approval, then build it.

Either way, Taskship picks the folder, gives the agent the task in a fixed prompt, records the launch in the task's [run history](history.md), and hands your terminal to the agent until it exits.

Claude Code is the agent Taskship launches today, and launching Codex is planned (see the [roadmap](roadmap.md)). The agent is chosen with `--agent` (`-a`); only `claude` exists for now, and it must be on your `PATH` (otherwise the command fails with exit code 6). Launching needs a terminal. The TUI launches the same way: press `r` on a task (see [Terminal UI](tui.md#launching-an-agent)).

To work with Taskship in sessions it didn't launch, install the [skill](skill.md). To tell agents how you like tasks written, add [conventions](conventions.md).

## Refine

An agent is only as good as the task it's given. `taskship refine PROJ-1` launches an agent to turn a rough idea into a task that's ready to work on, without implementing it:

```text
$ taskship refine PROJ-1
✓ Launching Claude to refine PROJ-1 in ~/code/myproject
```

The agent starts in the folder `taskship PROJ-1` would use (see [Which folder the agent starts in](#which-folder-the-agent-starts-in)), so it can check the task against the code. It reads Taskship's [guide to writing tasks](conventions.md#the-task-writing-guide) and [your conventions](conventions.md#your-conventions), if you have any, and asks about gaps only you can fill. Then it proposes a rewritten task: a clear title, the why and context, the scope, and acceptance criteria. It also suggests a split if the task is too big.

Refining looks at the project's other tasks too. The agent points out:

- **duplicates**, which it can merge into this task and archive;
- **tasks that fit with this one**, such as a bug in the same code, to do together;
- **tasks that should come first or after**, noted with a line such as "Blocked by PROJ-12" (Taskship has no dependency field).

The agent changes tasks only after you approve, using the same commands you would: `task edit --from`, `add`, `task archive`, and `comment add`. Taskship allows those commands in the session it launches, so the agent's permission check doesn't block them once you've approved.

Refining doesn't change the task's status, and it's recorded in the task's [run history](history.md) with the mode `refine`. When the task is ready, run it.

## Run

`taskship PROJ-1` (short for `taskship run PROJ-1`) launches an agent to work on the task: it plans first, waits for your approval, then builds it.

```text
$ taskship PROJ-1
✓ Moved PROJ-1 from todo to in_progress
✓ Launching Claude for PROJ-1 in ~/code/myproject
```

A task in `backlog` or `todo` moves to `in_progress` when the agent launches; Taskship does this itself, and moves it back if the agent can't be started. Other statuses don't change.

The agent starts with the task as its first message. It inspects the repository, assesses whether the task is ready, and asks questions if critical information is missing. Otherwise it proposes a plan and how it will check the work, and changes no files until you approve the plan. From there you work with the agent as usual. Work it finds outside the task, it offers to add as a new task.

When the work is done, the agent summarizes what changed and how it checked it, and suggests manual checks or a smoke test for bigger changes. Then it offers to add its summary as a comment on the task and to mark the task done. It does either only if you agree, and Taskship records which run did it (see [Run history](history.md)).

## Previewing a launch

`--dry-run` shows what a launch would do without starting anything: the agent, the folder it would start in, the Taskship commands it may run without asking, and the exact prompt it would receive. It works on both `taskship PROJ-1` and `taskship refine PROJ-1`.

## Which folder the agent starts in

The agent always starts at the project's folder, never in a deeper subfolder, so it can see the whole checkout. The folder is chosen in this order:

1. `--workspace PATH`, which must be in the task's project, or anywhere in a repository the project is set up in.
2. The current folder, under the same condition.
3. The project's only folder on this machine. If the project has several, the command fails with exit code 4 and asks for `--workspace`. If it has none, it says to run `taskship init` in the project's repository.

In a Git worktree, the agent starts at the same position in the worktree as the project's folder has in the main checkout. That is usually the worktree's top level. For a project set up at a monorepo subfolder such as `~/code/mono/api`, it is that subfolder of the worktree. A bare repository has no files to work on, so from outside its worktrees the command fails and says to run it from one of them.

## The prompt

Every launch starts from a prompt Taskship writes, not from whatever you'd type that day. That keeps runs consistent: the agent always gets the whole task, always checks it against the code, and always asks before changing anything, however you phrase things and whichever agent you use.

The prompt contains only the task's ref, project, title, description (acceptance criteria are part of the description), and comments, oldest first, followed by [your conventions](conventions.md#your-conventions) if you have any. It never contains other configuration or environment variables, so it can't carry credentials. It tells the agent how to run Taskship: `taskship`, or the full path of the `taskship` you launched when the one on your `PATH` is missing or a different file, such as a build in `./bin`.

The title and description, and the comments, are fenced and marked as untrusted, so text in a task can't pose as instructions. The rest of the prompt is the instructions for the job, refine or run, as described above.

## With Claude Code

Claude starts in plan mode, so the approval step is enforced by Claude itself, not just asked for in the prompt. Taskship also lets the session run the `taskship` commands its prompt mentions without stopping to ask (`--dry-run` lists them); everything else goes through your Claude permission settings as usual, and Taskship never writes to them. Neither is a security boundary, so keep your own permission settings in place.
