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

Each machine maps a project to a local repository checkout. A shared task does not imply a shared filesystem path.

## Plan-first dispatch

The default agent workflow reviews task readiness and proposes a plan before code changes. Explicit approval is required; agent-native permissions remain important.

## Leave no trace and worktree discovery

Taskship does not add metadata files to repositories. Projects are registered explicitly. Per-machine paths are configured outside repositories under `~/.taskship/`. When launched from an existing Git worktree of a registered project, Taskship recognizes and uses that worktree automatically.

## Initialization and skills

Taskship initializes its local SQLite database on first stateful use. `taskship init` is optional onboarding, not a required setup step. Agent skills are optional user-level installations: Taskship can dispatch a self-contained plan-first prompt even without a skill.
