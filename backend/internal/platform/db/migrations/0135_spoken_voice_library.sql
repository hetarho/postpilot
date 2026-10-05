-- +goose Up
CREATE TABLE spoken_voice_drafts (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL CHECK (revision > 0),
    name TEXT NOT NULL, description TEXT NOT NULL, preview_text TEXT NOT NULL,
    profile_json TEXT NOT NULL, qualification_session_id TEXT NOT NULL,
    generation_id TEXT NOT NULL DEFAULT '', selected_candidate_id TEXT NOT NULL DEFAULT '', confirmed_voice_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX spoken_voice_drafts_owner_idx ON spoken_voice_drafts(owner_id, updated_at DESC);

CREATE TABLE spoken_audio_assets (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL UNIQUE, sha256 TEXT NOT NULL, format TEXT NOT NULL,
    bytes INTEGER NOT NULL CHECK (bytes > 0 AND bytes <= 8388608),
    samples INTEGER NOT NULL CHECK (samples > 0 AND samples <= 13230000),
    sample_rate INTEGER NOT NULL CHECK (sample_rate = 44100), channels INTEGER NOT NULL CHECK (channels = 2),
    provenance_digest TEXT NOT NULL,
    created_at TEXT NOT NULL, revoked_at TEXT
);
CREATE INDEX spoken_audio_assets_owner_idx ON spoken_audio_assets(owner_id);

CREATE TABLE spoken_voice_candidates (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    draft_id TEXT NOT NULL REFERENCES spoken_voice_drafts(id) ON DELETE CASCADE,
    generation_id TEXT NOT NULL, supplier_handle TEXT NOT NULL,
    ordinal INTEGER NOT NULL CHECK (ordinal BETWEEN 0 AND 2),
    asset_id TEXT NOT NULL UNIQUE REFERENCES spoken_audio_assets(id) ON DELETE CASCADE,
    auditioned_at TEXT
);
CREATE INDEX spoken_voice_candidates_draft_idx ON spoken_voice_candidates(draft_id, generation_id);

CREATE TABLE spoken_voices (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL CHECK (revision > 0), name TEXT NOT NULL,
    description TEXT NOT NULL, preview_text TEXT NOT NULL,
    profile_json TEXT NOT NULL, supplier_handle TEXT NOT NULL,
    draft_id TEXT REFERENCES spoken_voice_drafts(id) ON DELETE SET NULL,
    candidate_id TEXT UNIQUE REFERENCES spoken_voice_candidates(id) ON DELETE SET NULL,
    sample_asset_id TEXT NOT NULL REFERENCES spoken_audio_assets(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL, removed_at TEXT
);
CREATE INDEX spoken_voices_owner_idx ON spoken_voices(owner_id, created_at DESC);

CREATE TABLE spoken_audio_access (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL REFERENCES spoken_audio_assets(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL, served_at TEXT
);
CREATE INDEX spoken_audio_access_expiry_idx ON spoken_audio_access(expires_at);

CREATE TABLE spoken_voice_requests (
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    operation TEXT NOT NULL, request_key TEXT NOT NULL, input_digest TEXT NOT NULL,
    result_id TEXT NOT NULL, result_revision INTEGER NOT NULL,
    PRIMARY KEY (owner_id, operation, request_key)
);

CREATE TABLE spoken_voice_uses (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id TEXT NOT NULL REFERENCES spoken_voices(id) ON DELETE CASCADE,
    voice_revision INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

-- Intents deliberately outlive account/metadata deletion and contain no audio.
CREATE TABLE spoken_audio_cleanup (
    id TEXT PRIMARY KEY, object_key TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL
);
-- +goose StatementBegin
CREATE TRIGGER spoken_audio_asset_cleanup BEFORE DELETE ON spoken_audio_assets
BEGIN
    INSERT OR IGNORE INTO spoken_audio_cleanup (id, object_key, created_at)
    VALUES (OLD.id, OLD.object_key, strftime('%Y-%m-%dT%H:%M:%f000000Z', 'now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER spoken_candidate_sample_cleanup AFTER DELETE ON spoken_voice_candidates
BEGIN
    DELETE FROM spoken_audio_assets WHERE id = OLD.asset_id
      AND NOT EXISTS (SELECT 1 FROM spoken_voices WHERE sample_asset_id = OLD.asset_id)
      AND NOT EXISTS (SELECT 1 FROM spoken_voice_candidates WHERE asset_id = OLD.asset_id);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER spoken_candidate_sample_cleanup;
DROP TRIGGER spoken_audio_asset_cleanup;
DROP TABLE spoken_audio_cleanup;
DROP TABLE spoken_voice_uses;
DROP TABLE spoken_voice_requests;
DROP TABLE spoken_audio_access;
DROP TABLE spoken_voices;
DROP TABLE spoken_voice_candidates;
DROP TABLE spoken_audio_assets;
DROP TABLE spoken_voice_drafts;
