-- +goose Up
-- Additive only. Legacy analyses retain unknown accepted-source freshness.
ALTER TABLE voice_samples ADD COLUMN content_revision INTEGER NOT NULL DEFAULT 1 CHECK(content_revision > 0);
ALTER TABLE voice_analyses ADD COLUMN source_versions_known INTEGER NOT NULL DEFAULT 0 CHECK(source_versions_known IN (0,1));
ALTER TABLE voice_analyses ADD COLUMN accepted_sources TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(accepted_sources));
ALTER TABLE voice_analyses ADD COLUMN accepted_material_snapshot TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(accepted_material_snapshot));
CREATE UNIQUE INDEX voice_samples_owned_revision ON voice_samples(user_id,voice_id,id,content_revision);
CREATE TABLE voice_sample_mutations (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 voice_id TEXT NOT NULL,
 sample_id TEXT NOT NULL,
 operation_key TEXT NOT NULL CHECK(operation_key <> ''),
 expected_content_revision INTEGER NOT NULL CHECK(expected_content_revision > 0),
 resulting_content_revision INTEGER NOT NULL CHECK(resulting_content_revision > 0),
 fingerprint TEXT NOT NULL,
 response TEXT NOT NULL CHECK(json_valid(response)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,operation_key),
 FOREIGN KEY(voice_id,user_id) REFERENCES voices(id,user_id) ON DELETE CASCADE
);
ALTER TABLE configuration_authoring_sessions ADD COLUMN saved_baseline TEXT CHECK(saved_baseline IS NULL OR json_valid(saved_baseline));
ALTER TABLE configuration_authoring_sessions ADD COLUMN working_source TEXT CHECK(working_source IS NULL OR json_valid(working_source));
ALTER TABLE configuration_authoring_sessions ADD COLUMN draft_state TEXT NOT NULL DEFAULT 'valid' CHECK(draft_state IN ('valid','incomplete','invalid'));
ALTER TABLE configuration_authoring_sessions ADD COLUMN has_unpublished_changes INTEGER NOT NULL DEFAULT 0 CHECK(has_unpublished_changes IN (0,1));
ALTER TABLE configuration_authoring_sessions ADD COLUMN saved_available INTEGER NOT NULL DEFAULT 0 CHECK(saved_available IN (0,1));
ALTER TABLE configuration_authoring_sessions ADD COLUMN publication_pending INTEGER NOT NULL DEFAULT 0 CHECK(publication_pending IN (0,1));
ALTER TABLE configuration_authoring_sessions ADD COLUMN target_conflict INTEGER NOT NULL DEFAULT 0 CHECK(target_conflict IN (0,1));
ALTER TABLE configuration_authoring_sessions ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_authoring_sessions ADD COLUMN candidate_count INTEGER NOT NULL DEFAULT 8 CHECK(candidate_count IN (2,4,8,16));
-- Existing selected snapshots remain readable; target baselines require a fresh owned read.
UPDATE configuration_authoring_sessions
SET working_source=json_extract(snapshot,'$.Selected'),
    has_unpublished_changes=CASE WHEN json_extract(snapshot,'$.Selected') IS NOT NULL AND json_extract(snapshot,'$.Saved') IS NULL THEN 1 ELSE 0 END,
    display_name=COALESCE(json_extract(snapshot,'$.Selected.Name'),''),
    saved_available=CASE WHEN target_id<>'' OR json_extract(snapshot,'$.Saved') IS NOT NULL THEN 1 ELSE 0 END;
CREATE INDEX configuration_authoring_summaries ON configuration_authoring_sessions(user_id,kind,saved_available,updated_at DESC,id DESC);
CREATE TABLE configuration_authoring_mutations (
 user_id TEXT NOT NULL,
 session_id TEXT NOT NULL,
 operation_key TEXT NOT NULL CHECK(operation_key <> ''),
 action TEXT NOT NULL CHECK(action IN ('patch','reset_chat','reset_baseline','select','save')),
 expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
 fingerprint TEXT NOT NULL,
 response TEXT NOT NULL CHECK(json_valid(response)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,operation_key),
 FOREIGN KEY(user_id,session_id) REFERENCES configuration_authoring_sessions(user_id,id) ON DELETE CASCADE
);
-- Material/generation setting changes increment this counter in the owning post context.
ALTER TABLE posts ADD COLUMN input_revision INTEGER NOT NULL DEFAULT 1 CHECK(input_revision > 0);

-- +goose Down
-- Forward-only: preserve paid records and revision evidence during binary rollback.
SELECT 1;
