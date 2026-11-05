# Quickstart (planned CLI)

```sh
# Install the Taskship binary using a supported package manager or release.
# No database setup is required: the first stateful local command initializes SQLite.

# Optional: install the Taskship skill in Claude user configuration
taskship skill install claude

# Create a project
taskship project add myproject --prefix PROJ

# Register a local checkout (outside the repository)
taskship workspace add myproject --path ~/code/myproject --default

# Capture work inside a project directory
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

`taskship init` is an optional onboarding wizard, not a prerequisite. Installation, help, and version checks do not initialize storage. Before dispatch, map the project to a local checkout. The agent must be installed separately. Commands are design proposals and may change.
