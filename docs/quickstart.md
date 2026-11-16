# Quickstart

## Install

```sh
curl -fsSL https://taskship.dev/install.sh | sh
```

The script downloads the `taskship` binary for macOS or Linux from the latest GitHub release, checks its checksum, and installs it in `~/.local/bin`. Nothing else is installed, and there is no database to set up: Taskship creates `~/.taskship` on first use.

> To install in another folder, set `TASKSHIP_INSTALL_DIR` for the script: `curl -fsSL https://taskship.dev/install.sh | TASKSHIP_INSTALL_DIR=~/bin sh`. `TASKSHIP_VERSION` installs a specific release the same way, such as `TASKSHIP_VERSION=v0.1.0`.

### Upgrading

```sh
taskship upgrade
```

`upgrade` checks for a newer release and shows the current and latest versions with a link to the release notes, then asks before changing anything (`--yes` skips the question, and is needed without a terminal). It downloads the release for your system, checks its checksum, runs it once in the folder it will live in, and only then replaces the `taskship` binary, following a symlink to the real file. If any step fails, or you press Ctrl+C, the old binary stays in place. Then the new version updates [the skill](skill.md) for each agent where it's installed and outdated; agents without the skill are left alone. Pre-releases are never installed, but a pre-release such as `v0.2.0-rc.1` upgrades to its release, `v0.2.0`.

Taskship sessions that are already running, such as the terminal UI or an agent run, keep using the old version. `upgrade` lists them with their process IDs so you can restart them.

A `taskship` built from source isn't replaced. If you installed it with `go install`, update it with `go install github.com/jhdenham/taskship/cmd/taskship@latest`; if you built it in a checkout, pull and rebuild.

To go back to an earlier release, reinstall it with the install script: `curl -fsSL https://taskship.dev/install.sh | TASKSHIP_VERSION=v0.1.0 sh`. If the newer release updated your data, the earlier one won't open it. Before each update, Taskship saves a copy of your data in `~/.taskship/backups/`; to restore it, close Taskship and replace `~/.taskship/taskship.db` with the copy. Changes made since then are lost.

## First steps

```sh
# Set up Taskship for a repository: asks for a project name and task prefix
cd ~/code/myproject
taskship init

# Capture work from anywhere in the repository
taskship add "Build a weekly habit tracker"

# Capture work from anywhere else
taskship add --project myproject "Build a weekly habit tracker"

# Review the task
taskship show PROJ-1

# Have an agent turn the rough idea into a task that's ready to work on
taskship refine PROJ-1

# Preview what an agent would be given
taskship PROJ-1 --dry-run

# Launch an agent on the task in this terminal (works without the skill)
taskship PROJ-1

# Browse and manage tasks on a board
taskship

# Optional: teach your agents to use Taskship in any session
taskship skill install all
```

`taskship init` sets up a project for the repository you're in and writes nothing to the repository itself. Installation, `--help`, and `--version` create no files. The skill goes in each agent's own folder for skills, never in the repository.

Taskship launches Claude Code today, which must be installed separately; launching Codex is planned, and the skill already works in both.

## See also

- [Projects](projects.md): set up a repository with `taskship init`, and how Taskship knows which project a folder belongs to.
- [Tasks](tasks.md): add, list, and edit tasks, with labels, priorities, and comments.
- [Launching agents](launching.md): refine tasks and run them, and what each launch gives the agent.
- [Terminal UI](tui.md): the board, the list, and every key.
