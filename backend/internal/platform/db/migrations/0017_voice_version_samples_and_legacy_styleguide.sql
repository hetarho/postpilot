-- +goose Up
-- Change 16, in two halves.
--
-- HALF ONE — the per-version generation snapshot. Every voice profile version may carry at
-- most one copy of the raw AI output of the last post generated under it, so a version can be
-- READ before it is adopted. (voice_id, version) is the primary key, which is what makes a
-- later generation REPLACE the snapshot rather than accumulate a second one; there is no
-- surrogate id, because a snapshot has no identity apart from the version it describes.
--
-- It is a COPY, not a reference: deleting the source post, regenerating it, editing it by hand
-- or reassigning it to another voice leaves the snapshot alone. `content` is opaque text as far
-- as the voice context is concerned — the voice domain never parses a post's content shape.
--
-- The (voice_id, user_id) foreign key is the shape 0016 gave voice_profile_versions: it puts
-- the row inside the per-voice, per-account partition, so no cross-voice or cross-account read
-- is expressible and a soft-deleted voice's snapshots go with the voice ([I4]).
CREATE TABLE voice_version_samples (
    voice_id   TEXT NOT NULL,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    version    INTEGER NOT NULL CHECK (version > 0),
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (voice_id, version),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);

-- HALF TWO — voice_profiles.styleguide goes.
--
-- It held the analysis model's output verbatim, and change 02 already copied that same text
-- into the structured profile's lexical description — so the prompt rendered it twice, once as
-- the structured profile and once appended under a `[Legacy manual guidance]` header. Its only
-- other job was to be the target of the corpus_version-guarded write that decides whether a
-- finished analysis is still the newest; corpus_version is its own column, so that guard
-- survives the drop.
--
-- `rules` STAYS. It is no longer editable and no longer reaches any RPC, but it is still the
-- target of `규칙으로 저장` on 글 다듬기 (voice.Service.AppendRule), which change 16 did not
-- ask to retire — see the job's build notes.
--
-- BEFORE the column goes, the ONE case that would lose real guidance is rescued. A voice that
-- was analysed before change 02 and never re-analysed since has its whole profile in this text
-- and NO published structured version (current_version = 0) — the copy of the analysis into the
-- structured profile's lexical description happens in the analyze handler, not in a migration,
-- so it never ran for such a voice. Dropping the column for those voices would silently reduce a
-- trained voice to an empty one.
--
-- So they get the version they never had: a v1 whose only content is that analysis text as the
-- lexical description, which is exactly the shape a described (seeded) voice publishes. The
-- snapshot is `json.Marshal` of voice.StructuredProfile — Go field names, missing fields
-- decoding to their zero values — so a minimal object is a valid profile. Every voice that HAS a
-- published version already carries its analysis there and needs nothing.
INSERT INTO voice_profile_versions (id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at)
SELECT lower(hex(randomblob(16))), p.user_id, p.voice_id, 1,
       json_object(
           'Empty', json('false'),
           'Lexical', json_object('Description', json_object('Value', p.styleguide, 'Source', 'analyzed'))
       ),
       'analysis', NULL, p.updated_at
FROM voice_profiles p
WHERE p.current_version = 0
  AND trim(p.styleguide) <> ''
  AND NOT EXISTS (SELECT 1 FROM voice_profile_versions v WHERE v.voice_id = p.voice_id);

UPDATE voice_profiles SET current_version = 1
WHERE current_version = 0
  AND trim(styleguide) <> ''
  AND EXISTS (SELECT 1 FROM voice_profile_versions v WHERE v.voice_id = voice_profiles.voice_id AND v.version = 1);

-- The table is REBUILT rather than `ALTER TABLE ... DROP COLUMN`ed, following 0009/0016, so the
-- surviving column set and both foreign keys are restated explicitly instead of inherited. It
-- carries no index of its own (0009 created none) and none is added here. This stays inside
-- goose's transaction: voice_profiles is a pure child — no table references it — so dropping it
-- cascades nothing.
CREATE TABLE voice_profiles_new (
    voice_id        TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rules           TEXT NOT NULL DEFAULT '',
    corpus_version  INTEGER NOT NULL DEFAULT 0,
    current_version INTEGER NOT NULL DEFAULT 0,
    updated_at      TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_profiles_new (voice_id, user_id, rules, corpus_version, current_version, updated_at)
SELECT voice_id, user_id, rules, corpus_version, current_version, updated_at FROM voice_profiles;
DROP TABLE voice_profiles;
ALTER TABLE voice_profiles_new RENAME TO voice_profiles;

-- +goose Down
-- The column comes back EMPTY. Nothing is copied back out of the structured profile, which
-- change 16 decided deliberately: the product is not open. The rescue above is not undone
-- either — a published version is real history, and 0016's Down already refuses to rewrite an
-- origin rather than invent one.
CREATE TABLE voice_profiles_old (
    voice_id        TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    styleguide      TEXT NOT NULL DEFAULT '',
    rules           TEXT NOT NULL DEFAULT '',
    corpus_version  INTEGER NOT NULL DEFAULT 0,
    current_version INTEGER NOT NULL DEFAULT 0,
    updated_at      TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO voice_profiles_old (voice_id, user_id, styleguide, rules, corpus_version, current_version, updated_at)
SELECT voice_id, user_id, '', rules, corpus_version, current_version, updated_at FROM voice_profiles;
DROP TABLE voice_profiles;
ALTER TABLE voice_profiles_old RENAME TO voice_profiles;

DROP TABLE voice_version_samples;
