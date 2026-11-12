# Concepts

## Projects and tasks

A project groups tasks and has a display prefix such as `PROJ`. A task has a title, a Markdown description, a workflow status, and a priority. Acceptance criteria are optional and live in the description, conventionally as a checklist under an `## Acceptance criteria` heading. Taskship doesn't enforce any structure for the description. Labels and comments are planned; the Terminal UI page describes them.

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

A run records one launch of an agent for a task: the agent, the machine, the folder, the agent's session, when it started and ended, and its exit code. A run is `running` while the agent is open, `ended` once it exits, and `failed` if the agent couldn't be started. Runs are separate from task workflow status: a run ending doesn't change the task.

`taskship runs PROJ-1` lists a task's runs, newest first (`--json` for scripts). To pick up a Claude session where it stopped, run `claude --resume SESSION` from the run's folder.

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
✓ Launching Claude for PROJ-1 in ~/code/myproject
```

Claude starts in plan mode, with the task as its first message. It reviews the repository and the task, proposes a plan, and asks you to approve the plan before it edits any files; that approval is Claude's own, not just a request in the prompt. From there you work in Claude as usual. Ctrl-C goes to Claude, and when Claude exits, `taskship` exits with Claude's exit code. Launching needs a terminal, and Claude Code must be on your `PATH` (otherwise the command fails with exit code 6). The agent is chosen with `--agent` (`-a`); only `claude` exists for now.

`taskship PROJ-1 --dry-run` shows what a run would do without launching anything: the agent, the folder it would start in, and the exact prompt it would receive.

The agent always starts at the project's folder, never in a deeper subfolder, so it can see the whole checkout. The folder is chosen in this order:

1. `--workspace PATH`, which must be in the task's project, or anywhere in a repository the project is set up in.
2. The current folder, under the same condition.
3. The project's only folder on this machine. If the project has several, the command fails with exit code 4 and asks for `--workspace`. If it has none, it says to run `taskship init` in the project's repository.

In a Git worktree, the agent starts at the same position in the worktree as the project's folder has in the main checkout. That is usually the worktree's top level. For a project set up at a monorepo subfolder such as `~/code/mono/api`, it is that subfolder of the worktree. A bare repository has no files to work on, so from outside its worktrees the command fails and says to run it from one of them.

The prompt contains only the task's ref, project, title, and description (acceptance criteria are part of the description), never configuration or environment variables, so it can't carry credentials. The title and description are fenced and marked as untrusted requirements, so text in a task can't pose as instructions. The prompt asks the agent to inspect the repository, assess whether the task is ready, ask questions if critical information is missing, and propose a plan without changing files until you approve it. Plan mode enforces the approval step inside Claude, but neither is a security boundary: keep using Claude's own permission settings.

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
