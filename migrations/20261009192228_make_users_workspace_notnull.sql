-- +goose Up
ALTER TABLE users
ALTER COLUMN workspace_id SET NOT NULL;

-- +goose Down
ALTER TABLE users
ALTER COLUMN workspace_id DROP NOT NULL;
