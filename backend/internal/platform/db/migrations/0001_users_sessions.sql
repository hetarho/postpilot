-- +goose Up
-- Accounts and sessions (PRD §5, plan 01). Timestamps are RFC3339 TEXT: SQLite has no
-- date type, and RFC3339 sorts lexicographically, so `expires_at < ?` is a plain string
-- comparison over an index.
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    -- argon2id in PHC string form, so the cost parameters can be raised later without
    -- a schema change (each row carries the parameters it was hashed with).
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE TABLE sessions (
    -- hex(sha256(raw token)). The raw token exists only in the cookie, so a leaked
    -- database yields no usable session.
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- +goose Down
DROP INDEX idx_sessions_expires_at;
DROP INDEX idx_sessions_user_id;
DROP TABLE sessions;
DROP TABLE users;
