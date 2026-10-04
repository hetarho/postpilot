-- +goose Up
CREATE TABLE spoken_voice_operations (
 id TEXT PRIMARY KEY,
 owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('voice_design','voice_confirm','voice_reuse_probe')),
 state TEXT NOT NULL CHECK(state IN ('reserved','queued','claimed','received','published','failed','unresolved','cancelled')),
 job_id TEXT NOT NULL DEFAULT '',
 idempotency_key TEXT NOT NULL,
 request_digest TEXT NOT NULL,
 scope_digest TEXT NOT NULL,
 candidate_id TEXT NOT NULL DEFAULT '',
 received_handle TEXT NOT NULL DEFAULT '',
 sample_asset_id TEXT REFERENCES spoken_audio_assets(id) ON DELETE SET NULL,
 snapshot_json TEXT NOT NULL,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 UNIQUE(owner_id,kind,idempotency_key)
);
CREATE INDEX spoken_operations_recovery_idx ON spoken_voice_operations(state);
CREATE INDEX spoken_operations_candidate_idx ON spoken_voice_operations(owner_id,candidate_id,kind);
-- Retain the exact audition while a supplier-confirmation outcome needs recovery.
DROP TRIGGER spoken_candidate_sample_cleanup;
-- +goose StatementBegin
CREATE TRIGGER spoken_candidate_sample_cleanup AFTER DELETE ON spoken_voice_candidates
BEGIN
 DELETE FROM spoken_audio_assets WHERE id=OLD.asset_id
 AND NOT EXISTS(SELECT 1 FROM spoken_voices WHERE sample_asset_id=OLD.asset_id)
 AND NOT EXISTS(SELECT 1 FROM spoken_voice_candidates WHERE asset_id=OLD.asset_id)
 AND NOT EXISTS(SELECT 1 FROM spoken_voice_operations WHERE sample_asset_id=OLD.asset_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER spoken_operation_sample_cleanup AFTER DELETE ON spoken_voice_operations
BEGIN
 DELETE FROM spoken_audio_assets WHERE id=OLD.sample_asset_id
 AND NOT EXISTS(SELECT 1 FROM spoken_voices WHERE sample_asset_id=OLD.sample_asset_id)
 AND NOT EXISTS(SELECT 1 FROM spoken_voice_candidates WHERE asset_id=OLD.sample_asset_id)
 AND NOT EXISTS(SELECT 1 FROM spoken_voice_operations WHERE sample_asset_id=OLD.sample_asset_id);
END;
-- +goose StatementEnd
CREATE TABLE spoken_probe_audio (
 owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 voice_id TEXT NOT NULL REFERENCES spoken_voices(id) ON DELETE CASCADE,
 input_digest TEXT NOT NULL,
 origin_operation_id TEXT NOT NULL REFERENCES spoken_voice_operations(id) ON DELETE CASCADE,
 origin_job_id TEXT NOT NULL,
 asset_id TEXT NOT NULL REFERENCES spoken_audio_assets(id) ON DELETE CASCADE,
 evidence_json TEXT NOT NULL,
 timing_json TEXT NOT NULL,
 PRIMARY KEY(owner_id,voice_id,input_digest)
);
-- +goose Down
DROP TABLE spoken_probe_audio;
DROP TRIGGER spoken_operation_sample_cleanup;
DROP TRIGGER spoken_candidate_sample_cleanup;
DROP INDEX spoken_operations_candidate_idx;
DROP INDEX spoken_operations_recovery_idx;
DROP TABLE spoken_voice_operations;
-- +goose StatementBegin
CREATE TRIGGER spoken_candidate_sample_cleanup AFTER DELETE ON spoken_voice_candidates
BEGIN
 DELETE FROM spoken_audio_assets WHERE id=OLD.asset_id
 AND NOT EXISTS(SELECT 1 FROM spoken_voices WHERE sample_asset_id=OLD.asset_id)
 AND NOT EXISTS(SELECT 1 FROM spoken_voice_candidates WHERE asset_id=OLD.asset_id);
END;
-- +goose StatementEnd
