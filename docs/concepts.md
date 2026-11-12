# Concepts

## Projects and tasks

A project groups tasks and has a display prefix such as `PROJ`. A task has a title, a Markdown description, a workflow status, a priority, and labels. Acceptance criteria are optional and live in the description, conventionally as a checklist under an `## Acceptance criteria` heading. Taskship doesn't enforce any structure for the description.

Labels such as `bug` or `infra` are free-form words of up to 50 characters, without spaces or commas, matched without regard to case (they're stored in lowercase). Add them with `taskship add "Fix export" --label bug,cli` (or repeat `--label`), list the tasks that have all of some labels with `taskship tasks --label bug`, and change them in a task's Markdown.

Comments are notes on a task, such as what you tried or what an agent found. They're numbered #1, #2, and so on in the order they were added, and a deleted comment's number isn't reused. They're stored separately from the description, so editing the description never changes them:

```sh
taskship comment add PROJ-1 "Tried bumping the timeout; still flaky."
taskship comment add PROJ-1            # write it in $VISUAL or $EDITOR
taskship comment list PROJ-1
taskship comment edit PROJ-1 2         # opens the comment in your editor
taskship comment delete PROJ-1 2       # asks first; --yes skips the question
```

`comment add` and `comment edit` also take the text as an argument, or from stdin with `-`. `show` prints a task's comments after its description, `show --json` includes them, and so does the prompt an agent receives.

`taskship task copy PROJ-3` creates a new task from an existing one, in the same project or, with `--project`, in another. The title, description, priority, and labels are copied; comments, runs, and the completion date stay with the original. The copy starts in `todo` (or `--status`), can get a new title with `--title`, and its first comment says `Copied from PROJ-3.`

`taskship add` and `taskship task copy` take `--edit` to open the new task in your editor right away, the same way `taskship task edit` does. The task is created first, so closing the editor without changes keeps it as it is, and invalid changes are kept in a file for `--from`. `--edit` needs a terminal and can't be combined with `--json`.

## Setting up a repository

Run `taskship init` in a repository (or pass its path) to start using Taskship there:

```text
$ taskship init
Setting up Taskship for ~/code/myproject
Taskship doesn't write anything to this repository. Everything lives in ~/.taskship.

? Project name (myproject):
? Task prefix (MYPT): PROJ

✓ Created project myproject. Tasks will be numbered PROJ-1, PROJ-2, and so on.
```

- The suggested name comes from the repository's `origin` remote (or its only remote), else the folder's name. The suggested prefix comes from the name: initials for several words (`my-cool-app` gives `MCA`), a short word whole (`web` gives `WEB`), or four letters spanning a longer word (`taskship` gives `TSKP`). It's only a suggestion; type any prefix you like.
- In a linked worktree, `init` sets up the repository's main checkout, and every worktree is recognized. In a bare repository with worktrees, the bare repository is set up.
- In a subfolder of a repository, `init` asks whether to set up the whole repository (the default) or only that folder, for example one part of a monorepo. A folder that is already part of a project can become a separate project too: `init` asks, and the default is no. Tasks added in that folder then go to the new project. In a worktree, the folder is recorded at the same place in the main checkout.
- In a folder that isn't a repository but contains some, such as `~/git`, `init` asks whether to set up the whole folder or a single repository in it (the default), and for a repository, lists the most recently changed ones that aren't set up yet. A folder that isn't a repository is set up as a whole, except the repositories inside it.
- If a project with the chosen name already exists, for example after cloning the repository again somewhere else, `init` offers to link the folder to it. Folders of that project that no longer exist are replaced.
- `--name`, `--prefix`, and `-y` (accept the defaults) let `init` run without a terminal. It refuses your home folder and the filesystem root.

Running `init` again in a folder that is set up just says which project it belongs to.

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

Taskship's internal SQLite/PostgreSQL store is one source. Future sources include GitHub Issues and Linear. External providers remain authoritative for their issue fields.

## Agent runs

A run records one launch of an agent for a task: the agent, the machine, the folder, the agent's session, when it started and ended, and its exit code. A run is `running` while the agent is open, `ended` once it exits, and `failed` if the agent couldn't be started. Runs are separate from task workflow status: a run ending doesn't change the task.

Runs are numbered per task: the first run of `PROJ-1` is run #1, and Taskship calls it, for example, "Claude (run #1)". `taskship runs PROJ-1` lists a task's runs, newest first (`--json` for scripts):

```text
$ taskship runs PROJ-1
RUN  STARTED           AGENT   STATE  EXIT  COMPLETED  FOLDER            SESSION
#2   2026-11-12 14:03  claude  ended  0     ✓          ~/code/myproject  6320422c-f7e9-4ccc-9db9-b2ff8d38d95e
#1   2026-11-12 13:40  claude  ended  130              ~/code/myproject  a3036f1a-1c58-4c6d-a349-a29c3f35f74d
```

COMPLETED marks the run whose agent marked the task done. To pick up a Claude session where it stopped, run `claude --resume SESSION` from the run's folder.

In a terminal, `tasks`, `runs`, `show`, and comment lists use color: a task's status is green when `done`, highlighted when `in_progress`, and muted in `backlog` or `canceled`; a run's state is green for exit 0 and red for a failure or another exit code; and labels, folders, sessions, field names, and comment headings are muted. Piped or redirected output has no color, and neither does output with `NO_COLOR` set. `show --md` and `--json` are never colored.

When a task moves to `done`, Taskship records when, and `show` prints it, such as `Completed:  2026-11-12 14:31:40 by Claude (run #2)`; the time is cleared if the task leaves `done` again. While an agent runs, its `TASKSHIP_RUN_ID` environment variable tells the `taskship` commands it runs which run they belong to: if the agent marks the task done, the task records that run as the one that completed it, and comments the agent adds record the run too, shown in their heading: `#2 · 2026-11-12 14:30 · Claude (run #2)`. Taskship only accepts a run that is still running and belongs to the same task, so a leftover value does nothing. `--json` output includes `completed_at`, `completed_run_id`, and a comment's `run_id`.

## Which project a folder belongs to

Without `--project`, `taskship add` uses the project of the current folder:

1. Inside a Git repository, only folders set up for the same repository match, including from a nested folder or a linked worktree elsewhere on disk. A folder that isn't a repository never claims a repository inside it.
2. Outside a Git repository, a set-up folder matches if the current folder is inside it.
3. If several projects match, the most specific folder wins. In a linked worktree, the current folder also counts as its position in the main checkout, so `~/worktrees/feature/api` belongs to the project set up for `~/code/mono/api`. If that still leaves several projects, the command fails with exit code 4 and asks for `--project`.
4. If nothing matches and you have no projects yet, `add` offers to set one up first:

   ```text
   $ taskship add "Fix export"
   This repository isn't set up with Taskship yet.
   Taskship doesn't write anything to this repository. Everything lives in ~/.taskship.
   ? Set up Taskship now? (Y/n):
   ? Project name (myproject):
   ? Task prefix (MYPT):

   ✓ Created project myproject.
   ✓ Created MYPT-1: Fix export
   Run "taskship tasks" to list this project's tasks.
   ```

   Otherwise, without a terminal, or with `--json`, it fails with exit code 2 and suggests `taskship init` or `--project`. Declining the offer adds nothing and exits 1. Taskship never falls back to a default project.

`taskship tasks` (or `taskship task list`) uses the same rules to list only the current folder's project. Outside any project it lists all projects, and `--all-projects` does so from anywhere. In a terminal, the first line says what is shown and how to change it:

```text
$ taskship tasks
Showing 2 tasks in myproject (this folder). Use --all-projects to see every project.

REF     STATUS       PRIORITY  TITLE
PROJ-1  todo         high      Build a weekly habit tracker
PROJ-2  in_progress  none      Fix export
```

When the output is piped, that line is left out so scripts get only the table; `--json` is never affected.

## Plan-first dispatch

`taskship PROJ-1` (short for `taskship run PROJ-1`) launches Claude Code on the task in your terminal:

```text
$ taskship PROJ-1
✓ Moved PROJ-1 from todo to in_progress
✓ Launching Claude for PROJ-1 in ~/code/myproject
```

A task in `backlog` or `todo` moves to `in_progress` when the agent launches; Taskship does this itself, and moves it back if the agent can't be started. Other statuses don't change. Claude starts in plan mode, with the task as its first message. It reviews the repository and the task, proposes a plan, and asks you to approve the plan before it edits any files; that approval is Claude's own, not just a request in the prompt. From there you work in Claude as usual. When the work is done, Claude summarizes what changed and how it checked it, suggests manual checks or a smoke test for bigger changes, and offers to add its summary as a comment on the task and to mark the task done. It does either only if you agree. Ctrl-C goes to Claude, and when Claude exits, `taskship` exits with Claude's exit code. Launching needs a terminal, and Claude Code must be on your `PATH` (otherwise the command fails with exit code 6). The agent is chosen with `--agent` (`-a`); only `claude` exists for now.

`taskship PROJ-1 --dry-run` shows what a run would do without launching anything: the agent, the folder it would start in, and the exact prompt it would receive.

The agent always starts at the project's folder, never in a deeper subfolder, so it can see the whole checkout. The folder is chosen in this order:

1. `--workspace PATH`, which must be in the task's project, or anywhere in a repository the project is set up in.
2. The current folder, under the same condition.
3. The project's only folder on this machine. If the project has several, the command fails with exit code 4 and asks for `--workspace`. If it has none, it says to run `taskship init` in the project's repository.

In a Git worktree, the agent starts at the same position in the worktree as the project's folder has in the main checkout. That is usually the worktree's top level. For a project set up at a monorepo subfolder such as `~/code/mono/api`, it is that subfolder of the worktree. A bare repository has no files to work on, so from outside its worktrees the command fails and says to run it from one of them.

The prompt contains only the task's ref, project, title, description (acceptance criteria are part of the description), and comments, oldest first, never configuration or environment variables, so it can't carry credentials. It tells the agent how to run Taskship: `taskship`, or the full path of the `taskship` you launched when the one on your `PATH` is missing or a different file, such as a build in `./bin`. The title and description, and the comments, are fenced and marked as untrusted, so text in a task can't pose as instructions. The prompt asks the agent to inspect the repository, assess whether the task is ready, ask questions if critical information is missing, and propose a plan without changing files until you approve it. Plan mode enforces the approval step inside Claude, but neither is a security boundary: keep using Claude's own permission settings.

## Leave no trace and worktree discovery

Taskship does not add metadata files to repositories. Projects are set up explicitly, normally with `taskship init`. Per-machine paths are stored outside repositories under `~/.taskship/`. When used from an existing Git worktree of a set-up repository, Taskship recognizes it from Git metadata (`git rev-parse --git-common-dir`), which is read-only and runs no repository hooks.

## Storage and skills

Taskship creates its local SQLite database on first use; there is no separate storage setup. Agent skills (planned) are optional user-level installations: Taskship can dispatch a self-contained plan-first prompt even without a skill.

## Advanced: managing workspaces

`taskship init` covers most setups. Underneath, each machine maps a project to its local folders, called workspaces. A shared task does not imply a shared filesystem path: workspaces are stored only in the machine's local database and are never synced, so the same project can live at different paths on different machines.

The `workspace` commands manage these mappings directly, for cases such as two separate clones of one project, or a folder linked without `init`:

```sh
taskship workspace add myproject --path ~/code/myproject   # --path defaults to the current folder
taskship workspace list                                   # paths that no longer exist are marked (missing)
taskship workspace remove ~/code/myproject                # unregisters; files are not touched
```

Register a repository's main checkout once. Taskship identifies the repository by its common Git directory, so every linked worktree of it is recognized from Git metadata when you use it. Worktrees are never stored, so creating and deleting them, by hand or with a tool, leaves nothing to clean up. A folder that isn't a Git repository can be registered too, and covers everything below it except Git repositories inside it, which are separate projects.
