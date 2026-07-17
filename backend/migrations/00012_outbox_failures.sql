-- +goose Up
ALTER TABLE notification_outbox ADD COLUMN failed_at TIMESTAMPTZ;
ALTER TABLE notification_outbox ADD COLUMN failure_reason TEXT;
CREATE INDEX idx_notification_outbox_failed ON notification_outbox (failed_at) WHERE failed_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_notification_outbox_failed;
ALTER TABLE notification_outbox DROP COLUMN IF EXISTS failure_reason;
ALTER TABLE notification_outbox DROP COLUMN IF EXISTS failed_at;
