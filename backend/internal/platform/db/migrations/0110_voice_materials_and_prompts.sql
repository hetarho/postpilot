-- +goose Up
-- VOICE r5 (T469): a voice's 학습 글 are pasted posts and answers to the product's shared
-- prompts, a photo prompt answered on the owner's own photo (VOICE-59, VOICE-60).
--
-- voice_samples is rebuilt the 0009 way to carry the kind: a post has no prompt and no
-- photo, an answer names its prompt and, for a photo prompt, the photo it was written on.
-- Every existing row is a pasted post. No trigger or child table names voice_samples, so
-- the rebuild needs no foreign-key pause.
CREATE TABLE voice_samples_new (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id     TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('post', 'answer')),
    prompt_key   TEXT,
    label        TEXT NOT NULL,
    body         TEXT NOT NULL,
    photo_key    TEXT,
    photo_width  INTEGER,
    photo_height INTEGER,
    created_at   TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    CHECK ((kind = 'post' AND prompt_key IS NULL AND photo_key IS NULL)
        OR (kind = 'answer' AND prompt_key IS NOT NULL)),
    CHECK ((photo_key IS NULL) = (photo_width IS NULL) AND (photo_key IS NULL) = (photo_height IS NULL))
);
INSERT INTO voice_samples_new (id, user_id, voice_id, kind, label, body, created_at)
SELECT id, user_id, voice_id, 'post', label, body, created_at FROM voice_samples;
DROP TABLE voice_samples;
ALTER TABLE voice_samples_new RENAME TO voice_samples;
CREATE INDEX voice_samples_voice_created_idx ON voice_samples(voice_id, created_at DESC, id DESC);
-- Each prompt holds one answer per voice; deleting the answer frees it (VOICE-60).
CREATE UNIQUE INDEX voice_samples_voice_prompt ON voice_samples(voice_id, prompt_key) WHERE prompt_key IS NOT NULL;

-- A photo prompt's photo is presigned before it is answered. The pending row names the key
-- so the orphan sweep can reclaim a PUT that was never answered, the way `uploads` does for
-- a post (POST-35, POST-40).
CREATE TABLE voice_photo_uploads (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    prompt_key  TEXT NOT NULL,
    object_key  TEXT NOT NULL UNIQUE,
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
CREATE INDEX voice_photo_uploads_expires_idx ON voice_photo_uploads(expires_at);

-- +goose Down
DROP TABLE voice_photo_uploads;
CREATE TABLE voice_samples_old (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id    TEXT NOT NULL,
    label       TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_samples_old (id, user_id, voice_id, label, body, created_at)
SELECT id, user_id, voice_id, label, body, created_at FROM voice_samples WHERE kind = 'post';
DROP TABLE voice_samples;
ALTER TABLE voice_samples_old RENAME TO voice_samples;
CREATE INDEX voice_samples_voice_created_idx ON voice_samples(voice_id, created_at DESC, id DESC);
