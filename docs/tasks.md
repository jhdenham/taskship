# Tasks

A task has a title, a Markdown description, a workflow status, a priority, and labels. Acceptance criteria are optional and live in the description, conventionally as a checklist under an `## Acceptance criteria` heading. Taskship doesn't enforce any structure for the description.

## Adding and listing tasks

```sh
taskship add "Fix export"                      # in the current folder's project
taskship add --project myproject "Fix export"  # from anywhere
taskship add "Fix export" --label bug --priority high
taskship tasks                                 # this folder's project
taskship tasks --status todo,in_progress --label bug
taskship tasks --all-projects
taskship show PROJ-1
```

Without `--project`, `add` and `tasks` use the project of the current folder (see [Which project a folder belongs to](projects.md#which-project-a-folder-belongs-to)). In a terminal, the first line of `tasks` says what is shown and how to change it:

```text
$ taskship tasks
Showing 2 tasks in myproject (this folder). Use --all-projects to see every project.

REF     STATUS       PRIORITY  TITLE
PROJ-1  todo         high      Build a weekly habit tracker
PROJ-2  in_progress  none      Fix export
```

When the output is piped, that line is left out so scripts get only the table; `--json` is never affected.

In a terminal, `tasks`, `runs`, `show`, and comment lists use color: a task's status is green when `done`, highlighted when `in_progress`, and muted in `backlog` or `canceled`; a run's state is green for exit 0 and red for a failure or another exit code; and labels, folders, sessions, field names, and comment headings are muted. Piped or redirected output has no color, and neither does output with `NO_COLOR` set. `show --md` and `--json` are never colored.

## Labels

Labels such as `bug` or `infra` are free-form words of up to 50 characters, without spaces or commas, matched without regard to case (they're stored in lowercase). Add them with `taskship add "Fix export" --label bug,cli` (or repeat `--label`), list the tasks that have all of some labels with `taskship tasks --label bug`, and change them in a task's Markdown.

## Comments

Comments are notes on a task, such as what you tried or what an agent found. They're numbered #1, #2, and so on in the order they were added, and a deleted comment's number isn't reused. They're stored separately from the description, so editing the description never changes them:

```sh
taskship comment add PROJ-1 "Tried bumping the timeout; still flaky."
taskship comment add PROJ-1            # write it in $VISUAL or $EDITOR
taskship comment list PROJ-1
taskship comment edit PROJ-1 2         # opens the comment in your editor
taskship comment delete PROJ-1 2       # asks first; --yes skips the question
```

`comment add` and `comment edit` also take the text as an argument, or from stdin with `-`. `show` prints a task's comments after its description, `show --json` includes them, and so does the prompt an agent receives.

## Copying tasks

`taskship task copy PROJ-3` creates a new task from an existing one, in the same project or, with `--project`, in another. The title, description, priority, and labels are copied; comments, runs, and the completion date stay with the original. The copy starts in `todo` (or `--status`), can get a new title with `--title`, and its first comment says `Copied from PROJ-3.`

`taskship add` and `taskship task copy` take `--edit` to open the new task in your editor right away, the same way `taskship task edit` does. The task is created first, so closing the editor without changes keeps it as it is, and invalid changes are kept in a file for `--from`. `--edit` needs a terminal and can't be combined with `--json`.

## Tasks as Markdown

Tasks are stored in SQLite, and Markdown is how you read and edit them by hand:

```markdown
---
ref: PROJ-1
project: myproject
status: todo
priority: high
labels: [habits, web]
---

# Build a weekly habit tracker

Track habits per week.

## Acceptance criteria

- [ ] Weekly view shows each habit
- [ ] Data persists across restarts
```

- `taskship show PROJ-1 --md` prints a task in this format.
- `taskship task edit PROJ-1` with no flags opens it in `$VISUAL` or `$EDITOR` (as does `--edit` on `add` and `task copy`). `--from FILE` (or `--from -` for stdin) applies a document without an editor, which is how scripts and agents edit tasks.
- `taskship export DIR` writes each task to `DIR/PROJ-1.md`. The files are a snapshot; editing them doesn't change the tasks.

`show --md` and `export` add a task's comments at the end, after a `<!-- taskship:comments ... -->` line, as a `## Comments` section with one `### #1 · 2026-11-12 14:03` heading per comment. That part is read-only: when a document is applied, everything from that line on is ignored, so the comments never become part of the description. `task edit` in your editor leaves the comments out.

When editing, `status`, `priority`, and `labels` can be changed, and `ref` and `project` can't. `labels` replaces the task's labels, and `labels: []` removes them all. The front matter keys are optional, and a missing key leaves that field unchanged. The `# Title` heading is required, and everything after it is the description. Only fields that differ from the current task are updated. If the editor's changes are invalid, they are kept in a temporary file and the error says how to apply them with `--from`.

## Task sources

Taskship's own store is the only source of tasks for now. GitHub Issues and Linear are planned as further sources, and they will stay authoritative for their issues' fields (see the [roadmap](roadmap.md)).
