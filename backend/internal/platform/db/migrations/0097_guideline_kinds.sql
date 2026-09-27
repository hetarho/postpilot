-- +goose NO TRANSACTION
-- +goose Up
-- 영상 지침 (GUIDE-2, GUIDE-4): a guideline and a guideline candidate carry a kind for good — a
-- post's 지침 or a clip's 영상 지침 — and every rule that counts or deduplicates them counts within
-- the kind (GUIDE-5, GUIDE-10). A clip guideline is scoped globally or to video templates, whose
-- links get a table of their own beside guideline_templates.
--
-- NO TRANSACTION plus an explicit PRAGMA, as 0022 and 0078 do: both tables change a UNIQUE
-- constraint, which SQLite can only do by rebuilding, and `guidelines` is the FK parent of
-- guideline_templates and guideline_fields with ON DELETE CASCADE. Dropping the old table with
-- foreign keys on would cascade every link away. The work still runs in one explicit transaction
-- and the integrity guard proves the graph before it commits. Every existing row becomes `post`.
PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE guidelines_new (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL DEFAULT 'post' CHECK (kind IN ('post','clip')),
    text       TEXT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('global','templates','fields')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, user_id),
    UNIQUE (user_id, kind, text)
);

INSERT INTO guidelines_new (id, user_id, kind, text, scope, created_at, updated_at)
SELECT id, user_id, 'post', text, scope, created_at, updated_at FROM guidelines;

DROP INDEX idx_guidelines_user_created;
DROP TABLE guidelines;
ALTER TABLE guidelines_new RENAME TO guidelines;
CREATE INDEX idx_guidelines_user_created ON guidelines(user_id, kind, created_at, id);

-- The candidate names where it was first seen: a post by slug, a clip project by id. Both are
-- plain columns with NO foreign key, for the reason 0023 gave post_slug: deleting the source must
-- leave the text and only drop the link (GUIDE-13).
CREATE TABLE guideline_candidates_new (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL DEFAULT 'post' CHECK (kind IN ('post','clip')),
    text          TEXT NOT NULL,
    post_slug     TEXT,
    clip_id       TEXT,
    status        TEXT NOT NULL CHECK (status IN ('pending','approved','dismissed')),
    occurrences   INTEGER NOT NULL DEFAULT 1,
    first_seen_at TEXT NOT NULL,
    last_seen_at  TEXT NOT NULL,
    UNIQUE (user_id, kind, text)
);

INSERT INTO guideline_candidates_new (id, user_id, kind, text, post_slug, clip_id, status,
                                      occurrences, first_seen_at, last_seen_at)
SELECT id, user_id, 'post', text, post_slug, NULL, status, occurrences, first_seen_at, last_seen_at
FROM guideline_candidates;

DROP INDEX idx_guideline_candidates_review;
DROP TABLE guideline_candidates;
ALTER TABLE guideline_candidates_new RENAME TO guideline_candidates;
CREATE INDEX idx_guideline_candidates_review
    ON guideline_candidates(user_id, kind, status, occurrences DESC, last_seen_at DESC);

-- A clip guideline's video templates. The composite FKs keep every link inside the account, and
-- deleting a video template cascades its links, leaving the guideline 적용 대상 없음 (GUIDE-14).
CREATE TABLE guideline_video_templates (
    guideline_id      TEXT NOT NULL,
    video_template_id TEXT NOT NULL,
    user_id           TEXT NOT NULL,
    PRIMARY KEY (guideline_id, video_template_id),
    FOREIGN KEY (guideline_id, user_id) REFERENCES guidelines(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (video_template_id, user_id) REFERENCES video_templates(id, user_id) ON DELETE CASCADE
);

CREATE INDEX idx_guideline_video_templates_template
    ON guideline_video_templates(user_id, video_template_id);

DROP TABLE IF EXISTS migration_0097_integrity_guard;
CREATE TABLE migration_0097_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0097_integrity_guard (problem)
SELECT 'rebuilding the guideline tables left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0097_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- Clip guidelines and clip candidates have nowhere to go in the old shape, so they are dropped;
-- every post row and link is kept.
PRAGMA foreign_keys=OFF;

BEGIN;

DROP INDEX idx_guideline_video_templates_template;
DROP TABLE guideline_video_templates;

CREATE TABLE guidelines_old (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('global','templates','fields')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, user_id),
    UNIQUE (user_id, text)
);
INSERT INTO guidelines_old (id, user_id, text, scope, created_at, updated_at)
SELECT id, user_id, text, scope, created_at, updated_at FROM guidelines WHERE kind = 'post';
DROP INDEX idx_guidelines_user_created;
DROP TABLE guidelines;
ALTER TABLE guidelines_old RENAME TO guidelines;
CREATE INDEX idx_guidelines_user_created ON guidelines(user_id, created_at, id);

CREATE TABLE guideline_candidates_old (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text          TEXT NOT NULL,
    post_slug     TEXT,
    status        TEXT NOT NULL CHECK (status IN ('pending','approved','dismissed')),
    occurrences   INTEGER NOT NULL DEFAULT 1,
    first_seen_at TEXT NOT NULL,
    last_seen_at  TEXT NOT NULL,
    UNIQUE (user_id, text)
);
INSERT INTO guideline_candidates_old (id, user_id, text, post_slug, status, occurrences, first_seen_at, last_seen_at)
SELECT id, user_id, text, post_slug, status, occurrences, first_seen_at, last_seen_at
FROM guideline_candidates WHERE kind = 'post';
DROP INDEX idx_guideline_candidates_review;
DROP TABLE guideline_candidates;
ALTER TABLE guideline_candidates_old RENAME TO guideline_candidates;
CREATE INDEX idx_guideline_candidates_review
    ON guideline_candidates(user_id, status, occurrences DESC, last_seen_at DESC);

DROP TABLE IF EXISTS migration_0097_down_integrity_guard;
CREATE TABLE migration_0097_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0097_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0097_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
