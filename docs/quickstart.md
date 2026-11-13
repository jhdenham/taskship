# Quickstart

## Install

```sh
curl -fsSL https://taskship.dev/install.sh | sh
```

The script downloads the `taskship` binary for macOS or Linux from the latest GitHub release, checks its checksum, and installs it in `~/.local/bin`. Nothing else is installed, and there is no database to set up: Taskship creates `~/.taskship` on first use.

> To install in another folder, set `TASKSHIP_INSTALL_DIR` for the script: `curl -fsSL https://taskship.dev/install.sh | TASKSHIP_INSTALL_DIR=~/bin sh`. `TASKSHIP_VERSION` installs a specific release the same way, such as `TASKSHIP_VERSION=v0.1.0`.

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

## Next

- [Terminal UI](tui.md): the board, the list, and every key.
- [Launching agents](launching.md): refine tasks and run them, and what each launch gives the agent.
- [Skill](skill.md): manage tasks from any agent session.
