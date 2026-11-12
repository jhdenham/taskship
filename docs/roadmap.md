# Roadmap

Taskship is in early development. This is the planned order of releases, not a set of commitments.

## v0.1: Local-first

Projects and tasks in a local SQLite database, with labels and comments, a CLI with JSON output, and a TUI for managing tasks and launching agents. `taskship init` sets up repositories without writing to them, and worktrees are recognized automatically. `taskship PROJ-1` launches Claude Code with a plan-first prompt, and an optional Claude skill improves the workflow without being required.

Task management (including labels and comments), setup, and launching Claude work today. The TUI and the skill are next.

## v0.2: Self-hosted server

An optional REST API backed by PostgreSQL or SQLite, with API tokens, so your laptop and a remote machine share the same tasks. Agents still run on the machine where you start them. Remote mode needs a connection; there is no offline sync.

## v0.3: External task providers

Browse and dispatch GitHub Issues and Linear issues directly, without copying them into Taskship. The provider stays the source of truth for the issue.

## v0.4: More agents

Codex support, structured plans, a fuller record of each agent run, and optional status and comment updates when a run finishes.

## Later

An MCP server, remote dispatch, worktree provisioning, an optional desktop app, offline sync across machines, and team features.
