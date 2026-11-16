-- +goose Up
-- VOICE r22 (T874): the learning chat asks first about one experience of the owner's own
-- (VOICE-78). An experience is a kind plus an optional name, private owner data (VOICE-58),
-- kept only so a resumed chat asks about the same experience. A thread answer is a
-- voice_samples row naming its experience and holds one answer per (experience, prompt).
-- Pre-alpha data is disposable: existing answers keep a NULL experience and stay situation
-- answers.
CREATE TABLE voice_experiences (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('food', 'cafe', 'beauty', 'stay', 'leisure', 'product')),
    name        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id) ON DELETE CASCADE
);
CREATE INDEX voice_experiences_voice_created_idx ON voice_experiences(voice_id, created_at DESC, id DESC);

ALTER TABLE voice_samples ADD COLUMN experience_id TEXT REFERENCES voice_experiences(id) ON DELETE CASCADE;

-- A situation still holds one answer per voice; a thread answer holds one per experience.
DROP INDEX voice_samples_voice_prompt;
CREATE UNIQUE INDEX voice_samples_voice_prompt ON voice_samples(voice_id, prompt_key)
    WHERE prompt_key IS NOT NULL AND experience_id IS NULL;
CREATE UNIQUE INDEX voice_samples_experience_prompt ON voice_samples(experience_id, prompt_key)
    WHERE experience_id IS NOT NULL;

-- +goose Down
DROP INDEX voice_samples_experience_prompt;
DROP INDEX voice_samples_voice_prompt;
DELETE FROM voice_samples WHERE experience_id IS NOT NULL;
ALTER TABLE voice_samples DROP COLUMN experience_id;
CREATE UNIQUE INDEX voice_samples_voice_prompt ON voice_samples(voice_id, prompt_key) WHERE prompt_key IS NOT NULL;
DROP TABLE voice_experiences;
