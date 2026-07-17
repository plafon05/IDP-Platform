-- +goose Up
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_task_resources_task_id ON task_resources(task_id);
CREATE INDEX IF NOT EXISTS idx_template_tasks_template_id ON template_tasks(template_id);
CREATE INDEX IF NOT EXISTS idx_tasks_category_id ON tasks(category_id);

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_category_id;
DROP INDEX IF EXISTS idx_template_tasks_template_id;
DROP INDEX IF EXISTS idx_task_resources_task_id;
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;
