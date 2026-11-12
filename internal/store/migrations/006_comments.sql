-- Task comments, numbered per task. last_comment_number keeps numbers from
-- being reused after a comment is deleted.
ALTER TABLE tasks ADD COLUMN last_comment_number INTEGER NOT NULL DEFAULT 0;

CREATE TABLE comments (
    id         TEXT PRIMARY KEY,
    task_id    TEXT NOT NULL REFERENCES tasks (id),
    number     INTEGER NOT NULL,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (task_id, number)
) STRICT;
