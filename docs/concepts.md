# Concepts

A short tour of the things Taskship works with. Each section links to the page with the details.

## Projects

A project groups tasks and has a prefix such as `PROJ`, so its tasks are numbered `PROJ-1`, `PROJ-2`, and so on. A project is usually one repository, or one part of a monorepo. See [Projects](projects.md).

## Tasks

A task has a title, a Markdown description, a status (`backlog`, `todo`, `in_progress`, `done`, or `canceled`), a priority (`none`, `low`, `medium`, `high`, or `urgent`), labels, and comments. Acceptance criteria are optional and live in the description. Tasks can be archived, and you can read and edit any task as a Markdown document. See [Tasks](tasks.md).

## Workspaces

A workspace is a folder on this machine where a project lives. `taskship init` records it, and Taskship uses it to tell which project you're in and where to start an agent. Workspaces are per machine: the same project can live at different paths on different machines. Git worktrees of a set-up repository are recognized without being recorded. See [Projects](projects.md).

## Agents and runs

Taskship launches a coding agent on a task for one of two jobs: to refine it (`taskship refine PROJ-1`), where the agent turns a rough idea into a task that's ready to work on, or to run it (`taskship PROJ-1`), where the agent plans and builds it. Each launch is recorded as a run, and Taskship knows which run finished a task. See [Launching agents](launching.md) and [Run history](history.md).

## The prompt

Every agent Taskship launches starts from a prompt Taskship writes: the task, fenced off as untrusted text, and instructions to check it against the code and plan before changing anything. Because the brief is the same every time, runs are consistent: you don't re-explain the task or remember to ask for a plan, and the result doesn't depend on how you phrased it that day or which agent you use. See [The prompt](launching.md#the-prompt).

## The skill, the guide, and your conventions

The optional Taskship skill teaches an agent to manage tasks in any session, not just the ones Taskship launches. Taskship's built-in guide tells every agent how to write a good task, and optional conventions in `~/.taskship/conventions.md` add your own rules on top. See [Skill](skill.md) and [Conventions](conventions.md).

## Storage

Everything Taskship knows lives in `~/.taskship` (or `$TASKSHIP_HOME`), in a SQLite database that's created by the first command that needs it. Installing Taskship, `--help`, and `--version` create no files.
