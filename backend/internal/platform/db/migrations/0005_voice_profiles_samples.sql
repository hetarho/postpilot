-- +goose Up
-- One account owns one editable profile. Samples remain separate because every corpus
-- change re-runs analysis over all of that account's source writing.
CREATE TABLE voice_profiles (
    user_id     TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    styleguide  TEXT NOT NULL DEFAULT '',
    rules       TEXT NOT NULL DEFAULT '',
    corpus_version INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL
);

CREATE TABLE voice_samples (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE INDEX voice_samples_user_created_idx
    ON voice_samples(user_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE voice_samples;
DROP TABLE voice_profiles;
