-- Runs are numbered per task (#1, #2, ...) in the order they started.
ALTER TABLE runs ADD COLUMN number INTEGER NOT NULL DEFAULT 0;

UPDATE runs SET number = (
    SELECT COUNT(*) FROM runs r
    WHERE r.task_id = runs.task_id
      AND (r.started_at < runs.started_at OR (r.started_at = runs.started_at AND r.id <= runs.id))
);

CREATE UNIQUE INDEX runs_task_number ON runs (task_id, number);
