# CLAUDE.md

## Project

Taskship is a local-first task manager and coding-agent dispatcher, written in Go. This is a monorepo: the Go CLI (and the future server, which is a subcommand of the same binary), the docs, and later the website.

Current state: see the status line in `README.md` for what is implemented. Detailed requirements are kept outside this public repo.

Never mention the maintainer's private projects in this repo. In examples, use `myproject` with the prefix `PROJ` (e.g. `PROJ-42`).

### Layout

- `cmd/taskship/` is the `main` package. Keep it thin and put logic in `internal/`.
- `internal/cli/` holds the Cobra command tree. Commands call application services and stay free of domain logic.
- `docs/` is user-facing documentation.
- `website/` is the Astro site for taskship.dev. It renders `docs/*.md` in place (never copy them); `website/src/nav.ts` sets the sidebar and must list every doc, or the build fails.

### Hard constraints (from the accepted decisions)

- **Go only** for the CLI, TUI, core and server: one `taskship` binary.
- **Leave no trace:** never write files into a managed repository. User state lives under `~/.taskship/`.
- **No side effects from metadata commands:** `--help` and `--version` must not create files. Local storage initializes lazily on the first stateful command.
- Never interpolate task text into shell commands. Spawn agents with argument arrays.
- Update docs in the same change as any CLI command that ships.

### Commands

```sh
go build -o bin/taskship ./cmd/taskship
go test ./...
go vet ./...
gofmt -l .
```

Website (Node 22+, see `website/.nvmrc`):

```sh
cd website && npm ci
npm run dev     # http://localhost:4321
npm run build
```

---

## Working guidelines

Adapted from [andrej-karpathy-skills](https://github.com/multica-ai/andrej-karpathy-skills). These lean toward caution over speed; use judgment on trivial tasks.

### 1. Think before coding

Don't assume. Don't hide confusion. Surface tradeoffs.

- State your assumptions. If you're unsure, ask.
- If a request can be read more than one way, lay out the readings instead of silently picking one.
- If a simpler approach exists, say so and push back.
- If something in the spec or plan is unclear or contradictory, stop and name the problem. Don't invent format rules.

### 2. Simplicity first

Write the minimum code that solves the problem. Nothing speculative.

- No features beyond what was asked, and no deferred roadmap items unless asked.
- No abstractions or interfaces for a single implementation.
- No configurability that wasn't requested.
- No error handling for impossible cases.
- If 200 lines could be 50, rewrite it. Would a senior Go engineer call this overcomplicated?

### 3. Surgical changes

Touch only what you must, and clean up only your own mess.

- Don't "improve" nearby code, comments, or formatting. Match the existing style.
- Mention unrelated dead code or bugs; don't fix them.
- Remove imports, variables, and functions that *your* change made unused.
- Every changed line should trace back to the request. (Taskship itself follows the same rule for Markdown files.)

### 4. Goal-driven execution

Define success criteria, then loop until they're verified.

- "Add X" becomes "write tests for X, then make them pass."
- "Fix a bug" becomes "write a failing test or fixture that reproduces it, then make it pass."
- "Refactor" becomes "tests pass before and after, and golden fixtures are unchanged."

For multi-step work, state a short plan first:

```
1. [step] → verify: [check]
2. [step] → verify: [check]
```

Done means `go test ./...`, `go vet ./...`, and `gofmt -l .` are all clean.
