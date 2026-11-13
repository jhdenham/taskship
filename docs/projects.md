# Projects

## Setting up a repository

Run `taskship init` in a repository (or pass its path) to start using Taskship there:

```text
$ taskship init
Setting up Taskship for ~/code/myproject
Taskship doesn't write anything to this repository. Everything lives in ~/.taskship.

? Project name (myproject):
? Task prefix (MYPT): PROJ

✓ Created project myproject. Tasks will be numbered PROJ-1, PROJ-2, and so on.

Next steps:
  taskship add "Fix the export bug"   Add a task from anywhere in this repository
  taskship tasks                      List this project's tasks
  taskship show PROJ-1                Show a task
  taskship PROJ-1 --dry-run           Preview what an agent would be given
```

- The suggested name comes from the repository's `origin` remote (or its only remote), else the folder's name. The suggested prefix comes from the name: initials for several words (`my-cool-app` gives `MCA`), a short word whole (`web` gives `WEB`), or four letters spanning a longer word (`taskship` gives `TSKP`). It's only a suggestion; type any prefix you like.
- In a linked worktree, `init` sets up the repository's main checkout, and every worktree is recognized. In a bare repository with worktrees, the bare repository is set up.
- In a subfolder of a repository, `init` asks whether to set up the whole repository (the default) or only that folder, for example one part of a monorepo. A folder that is already part of a project can become a separate project too: `init` asks, and the default is no. Tasks added in that folder then go to the new project. In a worktree, the folder is recorded at the same place in the main checkout.
- In a folder that isn't a repository but contains some, such as `~/git`, `init` asks whether to set up the whole folder or a single repository in it (the default), and for a repository, lists the most recently changed ones that aren't set up yet. A folder that isn't a repository is set up as a whole, except the repositories inside it.
- If a project with the chosen name already exists, for example after cloning the repository again somewhere else, `init` offers to link the folder to it. Folders of that project that no longer exist are replaced.
- `--name`, `--prefix`, and `-y` (accept the defaults) let `init` run without a terminal. It refuses your home folder and the filesystem root.

Running `init` again in a folder that is set up just says which project it belongs to.

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

`taskship tasks` (or `taskship task list`) uses the same rules to list only the current folder's project. Outside any project it lists all projects, and `--all-projects` does so from anywhere.

## Leave no trace and worktree discovery

Taskship does not add metadata files to repositories. Projects are set up explicitly, normally with `taskship init`. Per-machine paths are stored outside repositories under `~/.taskship/`. When used from an existing Git worktree of a set-up repository, Taskship recognizes it from Git metadata (`git rev-parse --git-common-dir`), which is read-only and runs no repository hooks.

## Advanced: managing workspaces

`taskship init` covers most setups. Underneath, each machine maps a project to its local folders, called workspaces. A shared task does not imply a shared filesystem path: workspaces are stored only in the machine's local database and are never synced, so the same project can live at different paths on different machines.

The `workspace` commands manage these mappings directly, for cases such as two separate clones of one project, or a folder linked without `init`:

```sh
taskship workspace add myproject --path ~/code/myproject   # --path defaults to the current folder
taskship workspace list                                   # paths that no longer exist are marked (missing)
taskship workspace remove ~/code/myproject                # unregisters; files are not touched
```

Register a repository's main checkout once. Taskship identifies the repository by its common Git directory, so every linked worktree of it is recognized from Git metadata when you use it. Worktrees are never stored, so creating and deleting them, by hand or with a tool, leaves nothing to clean up. A folder that isn't a Git repository can be registered too, and covers everything below it except Git repositories inside it, which are separate projects.
