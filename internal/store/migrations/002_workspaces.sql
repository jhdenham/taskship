-- Machine-local checkout paths for projects. Paths differ per machine, so
-- this table is never synced to a server. project_id has no foreign key
-- because in remote mode (v0.2) projects are not stored in this database.
-- git_common_dir identifies the repository, so every linked worktree of a
-- registered checkout is recognized without being stored.
CREATE TABLE workspaces (
    id             TEXT PRIMARY KEY,
    project_id     TEXT NOT NULL,
    path           TEXT NOT NULL UNIQUE,
    git_common_dir TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
) STRICT;

CREATE INDEX workspaces_project_id ON workspaces (project_id);
CREATE INDEX workspaces_git_common_dir ON workspaces (git_common_dir);
