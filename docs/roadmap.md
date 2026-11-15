# Roadmap

This is the plan for each release, written when the release was planned. It isn't a set of commitments, and it isn't updated as work ships: the [release notes](https://github.com/jhdenham/taskship/releases) say what each version actually includes.

## `v0.1`: Local-first

Projects and tasks in a local SQLite database, with no server or account. A CLI with JSON output and a terminal UI for managing tasks. `taskship init` sets up repositories without writing to them, and worktrees are recognized automatically. One command launches a coding agent on a task with a plan-first prompt, starting with Claude Code, and another launches it to refine a task until it's ready to work on. An optional skill teaches agents to manage tasks from any session.

## `v0.2`: Shared server

An optional self-hosted server, so your laptop and a remote machine share the same tasks. Each device keeps working from its own database when the server can't be reached, and syncs when it's back. Agents still run on the machine where you start them.

## `v0.3`: More agents

Launching Codex on tasks, choosing a default agent, plan mode as an option, structured plans, and a fuller record of each agent run.

## `v0.4`: External task providers

Browse and dispatch GitHub Issues and Linear issues directly, without copying them into Taskship. The provider stays the source of truth for the issue.

## Later

An MCP server, remote dispatch, worktree provisioning, an optional desktop app, bringing your own coding agent, plugins and hooks, and team features.
