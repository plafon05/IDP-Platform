-- +goose Up
ALTER TABLE idps
    ADD COLUMN archived_at TIMESTAMPTZ;

CREATE INDEX idx_idps_archived_at ON idps(archived_at);

-- +goose Down
DROP INDEX IF EXISTS idx_idps_archived_at;

ALTER TABLE idps
    DROP COLUMN IF EXISTS archived_at;
