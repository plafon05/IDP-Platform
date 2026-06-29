-- +goose Up
CREATE TABLE notification_reminders (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    kind VARCHAR(30) NOT NULL CHECK (kind IN ('deadline_soon', 'overdue')),
    due_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (task_id, kind, due_date)
);

-- +goose Down
DROP TABLE notification_reminders;
