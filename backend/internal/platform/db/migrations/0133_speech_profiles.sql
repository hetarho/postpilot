-- +goose Up
-- Speech curation has no membership in catalog_model_purposes or model selections.
CREATE TABLE speech_profiles (
    id TEXT PRIMARY KEY,
    current_revision INTEGER NOT NULL CHECK(current_revision > 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE speech_profile_revisions (
    profile_id TEXT NOT NULL REFERENCES speech_profiles(id),
    revision INTEGER NOT NULL CHECK(revision > 0),
    provider_id TEXT NOT NULL,
    design_model_id TEXT NOT NULL,
    speech_model_id TEXT NOT NULL,
    label TEXT NOT NULL,
    level TEXT NOT NULL CHECK(level IN ('free','value','balanced','premium','top')),
    enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
    binding_json TEXT NOT NULL,
    prices_json TEXT NOT NULL,
    voice_evidence TEXT NOT NULL DEFAULT '',
    export_evidence TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY(profile_id, revision)
);
CREATE TABLE speech_qualification_sessions (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    maximum_usd TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    FOREIGN KEY(profile_id, revision) REFERENCES speech_profile_revisions(profile_id, revision)
);
CREATE INDEX idx_speech_qualification_owner ON speech_qualification_sessions(owner_id, id);

-- +goose Down
DROP TABLE speech_qualification_sessions;
DROP TABLE speech_profile_revisions;
DROP TABLE speech_profiles;
