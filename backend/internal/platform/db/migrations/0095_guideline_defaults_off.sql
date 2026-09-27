-- +goose Up
-- GUIDE-16, GUIDE-43: the product's 기본 지침 are code constants, so the only thing an account
-- stores about them is a switch turned off. A row means that default is off for that account
-- and kind; no row means on, so a new account starts with every default on and a default
-- added later starts on for everyone.
CREATE TABLE guideline_defaults_off (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('post', 'clip')),
    default_key TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, kind, default_key)
);

-- +goose Down
DROP TABLE guideline_defaults_off;
