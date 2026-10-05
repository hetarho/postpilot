-- +goose Up
ALTER TABLE usage_unit_calls ADD COLUMN input_characters INTEGER NOT NULL DEFAULT 0 CHECK(input_characters >= 0 AND input_characters <= 1000);
ALTER TABLE usage_unit_calls ADD COLUMN input_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_projects ADD COLUMN dubbing_enabled INTEGER NOT NULL DEFAULT 0 CHECK(dubbing_enabled IN (0,1));
ALTER TABLE clip_projects ADD COLUMN dubbing_voice_id TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_projects ADD COLUMN dubbing_binding_digest TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE clip_projects DROP COLUMN dubbing_binding_digest;
ALTER TABLE clip_projects DROP COLUMN dubbing_voice_id;
ALTER TABLE clip_projects DROP COLUMN dubbing_enabled;
ALTER TABLE usage_unit_calls DROP COLUMN input_digest;
ALTER TABLE usage_unit_calls DROP COLUMN input_characters;
