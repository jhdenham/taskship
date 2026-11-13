# Run history

Every launch of an agent is recorded as a run: the agent, the machine, the folder, the agent's session, when it started and ended, and its exit code. A run is `running` while the agent is open, `ended` once it exits, and `failed` if the agent couldn't be started. Runs are separate from task workflow status: a run ending doesn't change the task.

Runs are numbered per task: the first run of `PROJ-1` is run #1, and Taskship calls it, for example, "Claude (run #1)". A run's mode says what it was for: `run` to work on the task, or `refine` to make it ready to work on (see [Launching agents](launching.md)). Both kinds share the task's numbering.

## Listing runs

`taskship runs PROJ-1` lists a task's runs, newest first (`--json` for scripts):

```text
$ taskship runs PROJ-1
RUN  STARTED           AGENT   MODE    STATE  EXIT  COMPLETED  FOLDER            SESSION
#3   2026-11-12 14:03  claude  run     ended  0     ✓          ~/code/myproject  6320422c-f7e9-4ccc-9db9-b2ff8d38d95e
#2   2026-11-12 13:40  claude  run     ended  130              ~/code/myproject  a3036f1a-1c58-4c6d-a349-a29c3f35f74d
#1   2026-11-12 11:15  claude  refine  ended  0                ~/code/myproject  0b9e33d1-58aa-4f7e-8c51-6a1f0d2c7e44

To resume a session, run "claude --resume SESSION" in its folder.
```

COMPLETED marks the run whose agent marked the task done. To pick up a Claude Code session where it stopped, run `claude --resume SESSION` from the run's folder.

## Which run finished a task

When a task moves to `done`, Taskship records when, and `show` prints it, such as `Completed:  2026-11-12 14:31:40 by Claude (run #2)`; the time is cleared if the task leaves `done` again.

While an agent runs, its `TASKSHIP_RUN_ID` environment variable tells the `taskship` commands it runs which run they belong to: if the agent marks the task done, the task records that run as the one that completed it, and comments the agent adds record the run too, shown in their heading: `#2 · 2026-11-12 14:30 · Claude (run #2)`. Taskship only accepts a run that is still running and belongs to the same task, so a leftover value does nothing. `--json` output includes `completed_at`, `completed_run_id`, and a comment's `run_id`.
