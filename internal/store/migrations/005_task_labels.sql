-- Task labels: lowercase, without whitespace or commas (validated in Go).
CREATE TABLE task_labels (
    task_id TEXT NOT NULL REFERENCES tasks (id),
    label   TEXT NOT NULL,
    PRIMARY KEY (task_id, label)
) STRICT;

CREATE INDEX task_labels_label ON task_labels (label);
