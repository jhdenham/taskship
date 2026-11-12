# Taskship

**From task to agent, in one command.**

Taskship is a local-first task manager and coding-agent dispatcher for developers. Organize projects and tasks from a CLI or TUI, then launch a coding agent such as Claude with the task's context. The agent reviews the task and proposes a plan before changing any code.

> **Status:** early development. Implemented so far: `taskship --version`, `taskship init`, `taskship project add|list|show|edit|archive|restore`, `taskship task add|list|show|edit|archive|restore` (with the shortcuts `add`, `tasks`, and `show`), `taskship workspace add|list|remove`, `taskship export`, `taskship TASK_REF` (short for `taskship run TASK_REF`; launches Claude Code in plan mode, or previews the prompt with `--dry-run`), and `taskship runs TASK_REF`. Tasks can be shown and edited as Markdown (`show --md`, `task edit` in `$EDITOR` or `--from`); see [docs/concepts.md](docs/concepts.md#tasks-as-markdown). Without `--project`, `add` and `tasks` use the project set up for the current folder or any Git worktree of the same repository (`tasks --all-projects` lists everything), and with no projects yet, `add` offers to set one up. Every other command described in `docs/` is planned, not implemented.
>
> Data is stored in `~/.taskship/` (override with `TASKSHIP_HOME`), which is created by the first command that needs it.

## Principles

- **Local-first.** SQLite by default, with no server or account. A self-hosted server is optional (v0.2).
- **Useful without agents.** Task management stands on its own.
- **Plan-first dispatch.** By default the agent assesses readiness and asks for approval before implementing.
- **Leave no trace.** Taskship never writes files into the repositories it manages. Its own state lives under `~/.taskship/`.
- **One Go binary.** The CLI, TUI and the future `taskship server` ship in a single executable.

## Roadmap

| Version | Focus |
|---|---|
| v0.1 | Standalone: SQLite, CLI/TUI, projects and tasks, plan-first Claude dispatch |
| v0.2 | Self-hosted Go API server with PostgreSQL, shared across machines |
| v0.3 | External task providers: GitHub Issues and Linear |
| v0.4 | Deeper agent integration: Codex, skills, structured plans |

## Repository layout

```text
cmd/taskship/       main package for the taskship binary
internal/cli/       Cobra command tree, exit codes, output
internal/core/      domain types and validation
internal/store/     SQLite persistence and embedded migrations
internal/workspace/ project and workspace resolution from directories and Git metadata
internal/dispatch/  agent run preparation: the plan-first prompt
docs/               user-facing documentation (planned behavior)
website/            Astro site for taskship.dev; renders docs/ as its docs pages
```

Planned additions: `internal/` packages for the domain, storage, dispatch and server code.

## Development

Requires Go (see `go.mod` for the version).

```sh
go build -o bin/taskship ./cmd/taskship
./bin/taskship --version

go test ./...
go vet ./...
gofmt -l .
```

Release builds set the version with `-ldflags "-X main.version=v0.1.0"`. `go install` builds report the module version.

### Website

The site in `website/` is built with [Astro](https://astro.build) and requires Node 22 or later (`website/.nvmrc` pins the version for `nvm use`).

```sh
cd website
npm ci
npm run dev       # local server at http://localhost:4321
npm run build     # static site in website/dist/
```

The docs pages are rendered from `docs/*.md` in place, so edit the docs there. `website/src/nav.ts` sets the sidebar order and must list every file in `docs/`; the build fails otherwise. CI builds the site on pushes to main and on pull requests.

## License

[Apache License 2.0](LICENSE)
