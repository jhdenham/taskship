# Terminal UI (planned v0.1)

Run `taskship` with no arguments, or `taskship tui`, to open a full-screen view of your project's tasks. Everything you can do to a task from the command line, you can do here: browse, filter, add, edit, comment, archive, and launch an agent. Both the keyboard and the mouse work.

The TUI needs a terminal. Without one (in a pipe or a script), `taskship` prints its help instead, and `taskship tui` fails with exit code 2.

## Which project it shows

The TUI opens on the project of the current folder, chosen the same way as for `taskship tasks`. Anywhere else, it opens the project picker. You can switch projects from inside the TUI at any time.

Setting up projects and their folders stays on the command line (`taskship init`, `taskship workspace`). If you have no projects yet, the TUI offers to set up the current repository: it closes, asks the `taskship init` questions, and opens again once the project exists.

## Views

- **Board:** one column per status (backlog, todo, in progress, done). Moving a task to another column changes its status. Canceled tasks are hidden unless you show them.
- **List:** the same tasks as a sortable table.
- **Detail:** opens when you select a task. It shows the task's fields, its description rendered as Markdown (including the acceptance criteria checklist), its comments, and the actions you can take.

Archived tasks are hidden unless you show them. The filter matches titles and refs, and can narrow by label, status, and priority.

## Editing

Status, priority, and labels change in place. To edit the title or description, the TUI opens the task as Markdown in `$VISUAL` or `$EDITOR`, the same way `taskship task edit` does, and returns to where you were when you close the editor. If your changes are invalid, the TUI says why and lets you reopen the editor with them.

## Launching an agent

Launch an agent from a task's detail view. If you have more than one agent installed, the TUI asks which one.

Before anything else happens, the TUI checks that the task's project has a folder on this machine and that the agent is installed. If not, it tells you in place. Otherwise the TUI closes, leaves a line such as `✓ Launching Claude for PROJ-42 in ~/code/myproject` in your terminal, and starts the agent exactly as `taskship PROJ-42` would. The agent has the whole terminal, and Ctrl-C goes to it. When the agent exits, you're back at your shell; run `taskship` to return to the TUI.

## Labels and comments

Labels change in place, like status and priority.

Comments are added, edited, and deleted from the task's detail view, as with `taskship comment` on the command line.
