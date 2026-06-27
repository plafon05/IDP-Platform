-- +goose Up
ALTER TABLE tasks ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_tasks_idp_active ON tasks(idp_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_idp_active;
ALTER TABLE tasks DROP COLUMN IF EXISTS deleted_at;
