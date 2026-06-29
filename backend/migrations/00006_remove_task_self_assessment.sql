-- +goose Up
ALTER TABLE tasks
    DROP COLUMN self_rating,
    DROP COLUMN self_comment;

-- +goose Down
ALTER TABLE tasks
    ADD COLUMN self_rating VARCHAR(20) CHECK (self_rating IN ('met', 'partially_met', 'not_met')),
    ADD COLUMN self_comment TEXT;
