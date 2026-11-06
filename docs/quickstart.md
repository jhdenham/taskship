# Quickstart (planned CLI)

```sh
# Install the Taskship binary using a supported package manager or release.
# No database setup is required: the first stateful local command initializes SQLite.

# Optional: install the Taskship skill in Claude user configuration
taskship skill install claude

# Set up Taskship for a repository: asks for a project name and task prefix
cd ~/code/myproject
taskship init

# Capture work from anywhere in the repository
taskship add "Build a weekly habit tracker"

# Capture work (from anywhere)
taskship add --project myproject "Build a weekly habit tracker"

# Review the task
taskship show PROJ-1

# Preview the agent context
taskship PROJ-1 --agent claude --dry-run

# Launch the agent in the current terminal (works without the skill)
taskship PROJ-1 --agent claude
```

`taskship init` is an optional onboarding wizard, not a prerequisite. Installation, help, and version checks do not initialize storage. `taskship init` writes nothing to the repository. The agent must be installed separately. Commands are design proposals and may change.
