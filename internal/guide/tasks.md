# Writing Taskship tasks

A task is ready when someone who wasn't in the conversation, a developer or
a coding agent, can pick it up, do it, and tell when it's done.

## Title

- State the outcome as an imperative, in under about 70 characters:
  "Add CSV export to the reports page", not "CSV" or "Export broken?".
- For a bug, name the wrong behavior: "Fix login failing after a password
  reset".

## Description

Markdown, in this order. Leave out a section that would be empty.

1. **Why:** the problem or goal in a sentence or two.
2. **Context:** what exists today, relevant files, commands or links, and
   what has been tried.
3. **Scope:** what's included, and what's left out when someone might
   assume otherwise.
4. **`## Acceptance criteria`:** a checklist (`- [ ] ...`) when "done" can be
   checked. Each item is something another person could observe or run: a
   behavior, an output, a passing command. Not "works well" or "clean code".
5. **Open questions:** anything unknown or assumed, marked as such.

For a bug, include the steps to reproduce, the expected and actual
behavior, and the environment when it matters.

## Size

- One task is one reviewable change: something an agent can plan and finish
  in one session.
- Split a task when its parts touch unrelated areas, could ship separately,
  or when one part has to land before the others. Each new task must stand
  on its own; don't write "see the other task" in place of context.
- Don't split further than is useful. One task per file is too small.

## Accuracy

- Don't invent requirements, numbers, designs, or deadlines. If something
  needed is unknown, ask, or list it under open questions.
- Check what you claim about the code: the files, commands and behavior you
  mention should exist.
- Keep the user's intent and wording. Tighten, don't reinterpret.

## Fields

- **Labels:** reuse the labels the project already has. Add a new one only
  when the user asks.
- **Priority:** none, low, medium, high, or urgent. Set it only when the user
  gives one.
- **Status:** backlog, todo, in_progress, done, or canceled. New tasks start
  in todo. Whether a task is done is the user's call.

## Related tasks

- Before adding a task, look for an existing one that covers the same work,
  and offer to update it instead.
- Taskship has no dependency field. Note links in the description, for
  example "Blocked by PROJ-12" or "Follow-up to PROJ-7".
