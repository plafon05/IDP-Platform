-- +goose Up
CREATE TABLE in_app_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(50) NOT NULL,
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    action_url VARCHAR(500),
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_in_app_notifications_user_created
    ON in_app_notifications (user_id, created_at DESC);
CREATE INDEX idx_in_app_notifications_user_unread
    ON in_app_notifications (user_id, created_at DESC)
    WHERE read_at IS NULL;

-- +goose Down
DROP TABLE in_app_notifications;
