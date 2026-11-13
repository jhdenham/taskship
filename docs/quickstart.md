# Quickstart

## Install

```sh
curl -fsSL https://taskship.dev/install.sh | sh
```

The script downloads the `taskship` binary for macOS or Linux from the latest GitHub release, checks its checksum, and installs it in `~/.local/bin`. Set `TASKSHIP_INSTALL_DIR` to install it in another folder, or `TASKSHIP_VERSION` (such as `v0.1.0`) to install a specific release. Nothing else is installed, and there is no database to set up: Taskship creates `~/.taskship` on first use.

> The first release isn't out yet. Until then, build Taskship from source with Go: `go install github.com/jhdenham/taskship/cmd/taskship@latest`.

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

# Have Claude Code turn the rough idea into a task that's ready to work on
taskship refine PROJ-1

# Preview what an agent would be given
taskship PROJ-1 --dry-run

# Launch Claude Code on the task in this terminal (works without the skill)
taskship PROJ-1

# Planned: install the optional Taskship skill in Claude's user configuration
taskship skill install claude
```

`taskship init` sets up a project for the repository you're in and writes nothing to the repository itself. Installation, `--help`, and `--version` create no files. Installing the skill is planned. Claude Code must be installed separately.
