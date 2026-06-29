-- +goose Up
CREATE TABLE notification_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_notification_outbox_pending
    ON notification_outbox (created_at)
    WHERE published_at IS NULL;

-- +goose Down
DROP TABLE notification_outbox;
