# Concepts

## Projects and tasks

A project groups tasks and has a display prefix such as `PROJ`. A task has a title, a Markdown description, a workflow status, and a priority. Acceptance criteria are optional and live in the description, conventionally as a checklist under an `## Acceptance criteria` heading. Taskship doesn't enforce any structure for the description.

## Tasks as Markdown

Tasks are stored in SQLite, and Markdown is how you read and edit them by hand:

```markdown
---
ref: PROJ-1
project: myproject
status: todo
priority: high
---
# Build a weekly habit tracker

Track habits per week.

## Acceptance criteria

- [ ] Weekly view shows each habit
- [ ] Data persists across restarts
```

- `taskship show PROJ-1 --md` prints a task in this format.
- `taskship task edit PROJ-1` with no flags opens it in `$VISUAL` or `$EDITOR`. `--from FILE` (or `--from -` for stdin) applies a document without an editor, which is how scripts and agents edit tasks.
- `taskship export DIR` writes each task to `DIR/PROJ-1.md`. The files are a snapshot; editing them doesn't change the tasks.

When editing, `status` and `priority` can be changed, and `ref` and `project` can't. The front matter keys are optional, and a missing key leaves that field unchanged. The `# Title` heading is required, and everything after it is the description. Only fields that differ from the current task are updated. If the editor's changes are invalid, they are kept in a temporary file and the error says how to apply them with `--from`.

## Task sources

Taskship's internal SQLite/PostgreSQL store is one source. Future sources include GitHub Issues and Linear. External providers remain authoritative for their issue fields.

## Agent runs

A run records an attempt to work on a task, including agent, machine, plan, and outcome where available. Runs are separate from task workflow status.

## Workspace mappings

Each machine maps a project to its local checkouts, called workspaces. A shared task does not imply a shared filesystem path: workspaces are stored only in the machine's local database and are never synced, so the same project can live at different paths on different machines.

```sh
taskship workspace add myproject --path ~/code/myproject   # --path defaults to the current directory
taskship workspace list                                   # paths that no longer exist are marked (missing)
taskship workspace remove ~/code/myproject                # unregisters; files are not touched
```

Register a repository's main checkout once. Taskship identifies the repository by its common Git directory, so every linked worktree of it is recognized from Git metadata when you use it. Worktrees are never stored, so creating and deleting them, by hand or with a tool, leaves nothing to clean up. A non-Git folder can be registered too, and covers everything below it.

Without `--project`, `taskship add` uses the project of the current directory:

1. Inside a Git repository, any workspace of the same repository matches, including from a nested folder or a linked worktree elsewhere on disk.
2. Otherwise, a workspace matches if the current directory is inside it.
3. If several projects match, the most specific workspace containing the current directory wins. If none contains it (for example, a worktree of a repository registered for two projects), the command fails with exit code 4 and asks for `--project`.
4. If nothing matches, the command fails and suggests `--project` or `taskship workspace add`. Taskship never falls back to a default project.

`taskship tasks` (or `taskship task list`) uses the same rules to list only the current directory's project. Outside any project it lists all projects, and `--all-projects` does so from anywhere. In a terminal, the first line says what is shown and how to change it:

```text
$ taskship tasks
Showing 2 tasks in myproject (this directory). Use --all-projects to see every project.

REF     STATUS       PRIORITY  TITLE
PROJ-1  todo         high      Build a weekly habit tracker
PROJ-2  in_progress  none      Fix export
```

When the output is piped, that line is left out so scripts get only the table; `--json` is never affected.

## Plan-first dispatch

The default agent workflow reviews task readiness and proposes a plan before code changes. Explicit approval is required; agent-native permissions remain important.

## Leave no trace and worktree discovery

Taskship does not add metadata files to repositories. Projects are registered explicitly. Per-machine paths are stored outside repositories under `~/.taskship/`. When launched from an existing Git worktree of a registered project, Taskship recognizes it from Git metadata (`git rev-parse --git-common-dir`), which is read-only and runs no repository hooks.

## Initialization and skills

Taskship initializes its local SQLite database on first stateful use. `taskship init` is optional onboarding, not a required setup step. Agent skills are optional user-level installations: Taskship can dispatch a self-contained plan-first prompt even without a skill.
