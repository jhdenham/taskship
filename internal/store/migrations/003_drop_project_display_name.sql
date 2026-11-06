-- Projects have one name (stored as slug); the separate display name is gone.
ALTER TABLE projects DROP COLUMN name;
