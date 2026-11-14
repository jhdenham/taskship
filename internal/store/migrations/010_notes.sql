-- Notes: thoughts that aren't tasks yet, numbered per project. run_id is the
-- agent run that wrote the note, if any. last_note_number keeps numbers from
-- being reused after a note is deleted.
ALTER TABLE projects ADD COLUMN last_note_number INTEGER NOT NULL DEFAULT 0;

CREATE TABLE notes (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects (id),
    number     INTEGER NOT NULL,
    body       TEXT NOT NULL,
    run_id     TEXT REFERENCES runs (id),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (project_id, number)
) STRICT;
