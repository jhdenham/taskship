-- When a task was last completed (status done), and the agent run that
-- completed it, if any. Comments can come from an agent run too.
ALTER TABLE tasks ADD COLUMN completed_at TEXT;
ALTER TABLE tasks ADD COLUMN completed_run_id TEXT REFERENCES runs (id);
ALTER TABLE comments ADD COLUMN run_id TEXT REFERENCES runs (id);
