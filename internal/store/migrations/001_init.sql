CREATE TABLE projects (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    task_prefix TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    archived_at TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
) STRICT;

CREATE TABLE tasks (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES projects (id),
    sequence_number INTEGER NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'todo'
        CHECK (status IN ('backlog', 'todo', 'in_progress', 'done', 'canceled')),
    priority        TEXT NOT NULL DEFAULT 'none'
        CHECK (priority IN ('none', 'low', 'medium', 'high', 'urgent')),
    archived_at     TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE (project_id, sequence_number)
) STRICT;
