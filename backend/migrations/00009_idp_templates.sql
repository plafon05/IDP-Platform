-- +goose Up
ALTER TABLE idp_templates ADD COLUMN goals TEXT, ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE template_tasks ADD COLUMN due_offset_days INTEGER CHECK (due_offset_days BETWEEN 0 AND 3650);
CREATE TABLE template_competencies (
    template_id UUID NOT NULL REFERENCES idp_templates(id) ON DELETE CASCADE,
    competency_id UUID NOT NULL REFERENCES competencies(id),
    target_level SMALLINT NOT NULL CHECK (target_level BETWEEN 1 AND 4),
    PRIMARY KEY (template_id,competency_id)
);
CREATE INDEX idx_idp_templates_creator ON idp_templates(creator_id);

-- +goose Down
DROP INDEX idx_idp_templates_creator;
DROP TABLE template_competencies;
ALTER TABLE template_tasks DROP COLUMN due_offset_days;
ALTER TABLE idp_templates DROP COLUMN goals, DROP COLUMN updated_at;
