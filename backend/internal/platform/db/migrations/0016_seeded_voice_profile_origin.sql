-- +goose Up
-- Described initial voices (change 14). A voice created from a written description of the
-- wanted register publishes its first profile version from that description, so the version
-- history needs one more origin: 'seed'.
--
-- SQLite cannot alter a CHECK constraint, so the table is rebuilt. Unlike 0009 this stays
-- inside goose's transaction and leaves `PRAGMA foreign_keys` alone: voice_profile_versions
-- is a pure child — no table references it — so dropping it cascades nothing.
CREATE TABLE voice_profile_versions_new (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    version     INTEGER NOT NULL CHECK (version > 0),
    snapshot    TEXT NOT NULL,
    origin      TEXT NOT NULL CHECK (origin IN ('analysis','seed','manual','restore','rule','confirmation')),
    restored_from_version INTEGER,
    created_at  TEXT NOT NULL,
    UNIQUE (voice_id, version),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_profile_versions_new (id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at)
SELECT id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at FROM voice_profile_versions;
DROP TABLE voice_profile_versions;
ALTER TABLE voice_profile_versions_new RENAME TO voice_profile_versions;
CREATE INDEX voice_versions_voice_version ON voice_profile_versions(voice_id, version DESC);

-- +goose Down
-- The reverse rebuild drops any seeded version rather than rewriting its origin: a snapshot
-- labelled 'analysis' would claim evidence that was never measured.
CREATE TABLE voice_profile_versions_old (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    version     INTEGER NOT NULL CHECK (version > 0),
    snapshot    TEXT NOT NULL,
    origin      TEXT NOT NULL CHECK (origin IN ('analysis','manual','restore','rule','confirmation')),
    restored_from_version INTEGER,
    created_at  TEXT NOT NULL,
    UNIQUE (voice_id, version),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_profile_versions_old (id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at)
SELECT id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at
FROM voice_profile_versions WHERE origin <> 'seed';
DROP TABLE voice_profile_versions;
ALTER TABLE voice_profile_versions_old RENAME TO voice_profile_versions;
CREATE INDEX voice_versions_voice_version ON voice_profile_versions(voice_id, version DESC);
