-- +goose Up
-- Email identity remains nullable for operator-created legacy accounts. SQLite's unique
-- indexes permit multiple NULL values while still enforcing one owner per real address
-- and one account per Google subject.
ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN email_verified_at TEXT;
ALTER TABLE users ADD COLUMN email_unreachable_at TEXT;
ALTER TABLE users ADD COLUMN failed_logins INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN locked_until TEXT;
ALTER TABLE users ADD COLUMN google_subject TEXT;

CREATE UNIQUE INDEX idx_users_email ON users(email);
CREATE UNIQUE INDEX idx_users_google_subject ON users(google_subject);

-- Verification and reset credentials are revocable rows rather than self-contained signed
-- tokens. Only their SHA-256 hashes are stored; the raw token exists in one emailed URL.
CREATE TABLE auth_links (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    TEXT NOT NULL CHECK (purpose IN ('verify_email','reset_password')),
    email      TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at    TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_auth_links_user_purpose ON auth_links(user_id, purpose);

-- +goose Down
DROP INDEX idx_auth_links_user_purpose;
DROP TABLE auth_links;
DROP INDEX idx_users_google_subject;
DROP INDEX idx_users_email;
ALTER TABLE users DROP COLUMN google_subject;
ALTER TABLE users DROP COLUMN locked_until;
ALTER TABLE users DROP COLUMN failed_logins;
ALTER TABLE users DROP COLUMN email_unreachable_at;
ALTER TABLE users DROP COLUMN email_verified_at;
ALTER TABLE users DROP COLUMN email;
