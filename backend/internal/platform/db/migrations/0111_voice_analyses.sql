-- +goose Up
-- VOICE r5 (T471): a voice's analysis is its fingerprint (VOICE-6, VOICE-24), kept as the
-- current one and at most one previous (VOICE-30). The versioned profile, its per-version
-- generation snapshots and the per-field overrides leave the product.
--
-- Nothing is converted: the old profiles are a different kind of analysis, so every voice reads
-- as not made until its owner presses 말투 만들기. Only a made voice may be the 기본 (VOICE-2),
-- so no voice is one any more, and a new post starts on 말투 없음.
CREATE TABLE voice_analyses (
    voice_id      TEXT NOT NULL,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slot          TEXT NOT NULL CHECK (slot IN ('current', 'previous')),
    snapshot      TEXT NOT NULL CHECK (json_valid(snapshot)),
    material_ids  TEXT NOT NULL CHECK (json_valid(material_ids)),
    analyze_model TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    PRIMARY KEY (voice_id, slot),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);

DROP TABLE voice_version_samples;
DROP TABLE voice_manual_overrides;
DROP TABLE voice_profile_versions;
DROP TABLE voice_profiles;

UPDATE voices SET is_default = 0 WHERE is_default = 1;

-- +goose Down
-- Retirement is irreversible: the versioned profiles are gone, and lowering the recorded
-- version must not bring back tables no code reads.
SELECT 1;
