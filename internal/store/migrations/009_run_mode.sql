-- A run is a launch to work on the task ("run") or to refine it ("refine").
ALTER TABLE runs ADD COLUMN mode TEXT NOT NULL DEFAULT 'run';
