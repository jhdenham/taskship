-- Agent runs launched from this machine. task_ref is the ref at launch time;
-- workspace_path and machine are local execution details.
CREATE TABLE runs (
    id             TEXT PRIMARY KEY,
    task_id        TEXT NOT NULL REFERENCES tasks (id),
    task_ref       TEXT NOT NULL,
    agent          TEXT NOT NULL,
    machine        TEXT NOT NULL,
    workspace_path TEXT NOT NULL,
    session_id     TEXT NOT NULL DEFAULT '',
    state          TEXT NOT NULL
        CHECK (state IN ('starting', 'running', 'ended', 'failed')),
    exit_code      INTEGER,
    started_at     TEXT NOT NULL,
    ended_at       TEXT
) STRICT;

CREATE INDEX runs_task_id ON runs (task_id);
