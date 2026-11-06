# Quickstart

```sh
# Install the Taskship binary (release packages are planned).
# No database setup is needed: Taskship creates ~/.taskship on first use.

# Set up Taskship for a repository: asks for a project name and task prefix
cd ~/code/myproject
taskship init

# Capture work from anywhere in the repository
taskship add "Build a weekly habit tracker"

# Capture work from anywhere else
taskship add --project myproject "Build a weekly habit tracker"

# Review the task
taskship show PROJ-1

# Preview what an agent would be given
taskship PROJ-1 --dry-run

# Planned: launch the agent in the current terminal (works without the skill)
taskship PROJ-1 --agent claude

# Planned: install the optional Taskship skill in Claude's user configuration
taskship skill install claude
```

`taskship init` sets up a project for the repository you're in and writes nothing to the repository itself. Installation, `--help`, and `--version` create no files. Launching an agent and installing the skill are planned; the agent must be installed separately.
