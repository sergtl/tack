-- +goose Up
CREATE TABLE sessions (
    token_hash      TEXT PRIMARY KEY,
    user_id         INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at      TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE sessions;