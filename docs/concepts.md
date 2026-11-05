# Concepts

## Projects and tasks

A project groups tasks and has a display prefix such as `PROJ`. A task has a title, description, workflow status, priority, and optional acceptance criteria.

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
