-- +goose Up
CREATE TYPE issue_status AS ENUM(
    'todo',
    'in_progress',
    'completed',
    'canceled'
);
ALTER TABLE issues
ALTER COLUMN status TYPE issue_status USING status::issue_status;
ALTER TABLE issues
ALTER COLUMN status
SET DEFAULT 'todo';
-- +goose Down
ALTER TABLE issues
ALTER COLUMN status DROP DEFAULT;
ALTER TABLE issues
ALTER COLUMN status TYPE VARCHAR(64) USING status::text;
DROP TYPE issue_status;