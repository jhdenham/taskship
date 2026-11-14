# Notes

Notes hold thoughts that aren't tasks yet: an idea, a snippet worth keeping, something an agent noticed. They keep your backlog for work you mean to do, and a note that turns out to be real work becomes a task.

A note belongs to a project and is plain text, with no title, status, or labels. Notes are numbered #1, #2, and so on per project, and a deleted note's number isn't reused.

## Adding and listing notes

```sh
taskship note add "Try a compact board layout"
pbpaste | taskship note add -           # from stdin
taskship note add                       # write it in $VISUAL or $EDITOR
taskship note list                      # newest first
taskship note edit 3                    # opens the note in your editor
taskship note delete 3                  # asks first; --yes skips the question
```

Without `--project`, notes go to the project set up for the current folder. `note list --json` prints them for scripts and agents.

## Notes from agents

An agent working on a task often notices something outside it. Instead of losing it in the session or adding a task nobody asked for, it offers to leave a note (see [Launching agents](launching.md#run)). A note added by an agent that Taskship launched records the run, and `note list` shows it: `#4 · 2026-11-14 12:10 · Claude (run #1)`. Notes you write have no author.

## Turning a note into a task

In the [terminal UI](tui.md#notes), the Notes view shows notes as cards, and `N` shows them in a lane on the board. `t` on a note opens a box with the note as the new task's description; write a title and save, and the task goes to backlog and the note is removed.

## Exporting notes

`taskship export DIR` writes each project's notes, newest first, to `DIR/notes/myproject.md`, next to the task files. Archived projects and projects without notes get no file. Like the task files, it's a snapshot; editing it doesn't change the notes.
