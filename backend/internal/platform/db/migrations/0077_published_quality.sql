-- +goose Up
-- The published-status delta's one additive migration: every new column, index and table
-- except the guidelines rebuild, so no later task of the delta adds a migration of its own.
-- Nothing is rebuilt, backfilled or seeded, and no current read or write names any of it.
--
-- Owners, the tasks that first read or write each part:
--   T329  posts.published_url, posts.published_at and posts_user_published_idx
--   T337  posts.field and posts.quality_rules
--   T341  posts.content_nouns and posts.replacement_candidates
--   T335  post_measurements and field_phrase_lists
--   T333  templates.title_area
--
-- `posts` carries no CHECK on `status`, so 'published' needs no schema change. A nullable
-- column is written without an explicit NULL, as every earlier migration writes one: sqlc
-- reads `TEXT NULL` as no type at all and generates interface{} for it.

-- A post's Naver Blog address and when it was recorded. The two are set and cleared
-- together (POST-73, POST-75), and the CHECK on the second column says so for both.
ALTER TABLE posts ADD COLUMN published_url TEXT;
ALTER TABLE posts ADD COLUMN published_at TEXT CHECK ((published_url IS NULL) = (published_at IS NULL));

-- The post's blog field as its ASCII id. NULL is none. There is no CHECK: the product's
-- list lives in code, and its parser owns validity (QUAL-23).
ALTER TABLE posts ADD COLUMN field TEXT;

-- JSON written beside the content by the write pass (GEN-53, GEN-55) and the quality
-- ticks saved as a generation option (POST-81). NULL is "never written".
ALTER TABLE posts ADD COLUMN content_nouns TEXT CHECK (content_nouns IS NULL OR json_valid(content_nouns));
ALTER TABLE posts ADD COLUMN replacement_candidates TEXT CHECK (replacement_candidates IS NULL OR json_valid(replacement_candidates));
ALTER TABLE posts ADD COLUMN quality_rules TEXT CHECK (quality_rules IS NULL OR json_valid(quality_rules));

-- The account's published window, newest first (QUAL-39). SQLite uses a partial index only
-- when the read repeats its WHERE, so every window read names status = 'published'.
CREATE INDEX posts_user_published_idx ON posts(user_id, published_at) WHERE status = 'published';

-- A post's self-only numbers, stored against the revision they describe (QUAL-4). Every read
-- is by slug, through the primary key, so there is no secondary index; the composite key
-- keeps a row on its own account's post and deletes it with that post (POST-85).
CREATE TABLE post_measurements (
    post_slug            TEXT PRIMARY KEY,
    user_id              TEXT NOT NULL,
    content_revision     INTEGER NOT NULL,
    measure_version      INTEGER NOT NULL,
    char_count           INTEGER NOT NULL,
    photo_count          INTEGER NOT NULL,
    distinct_block_types INTEGER NOT NULL,
    avg_sentence_length  REAL,
    repetition_share     REAL,
    top_noun             TEXT,
    title_relevance      REAL,
    computed_at          TEXT NOT NULL,
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE
);

-- Each blog field's phrase list: product-owned data the daily batch refreshes, never an
-- owner's (QUAL-22), so it has no user_id. It starts empty (QUAL-41, QUAL-42).
CREATE TABLE field_phrase_lists (
    field           TEXT PRIMARY KEY,
    phrases         TEXT NOT NULL CHECK (json_valid(phrases)),
    corpus_size     INTEGER NOT NULL,
    refreshed_at    TEXT,
    next_refresh_at TEXT NOT NULL
);

-- The template's optional title area (TMPL-2). The default is the no-title-area state every
-- existing template already has (TMPL-52), so nothing is backfilled.
ALTER TABLE templates ADD COLUMN title_area TEXT NOT NULL DEFAULT '';

-- +goose Down
-- A published post always holds its finalized revision, so finalized is exactly what it
-- meant before this migration.
UPDATE posts SET status = 'finalized' WHERE status = 'published';
ALTER TABLE templates DROP COLUMN title_area;
DROP TABLE field_phrase_lists;
DROP TABLE post_measurements;
-- Before published_at, which the index names.
DROP INDEX posts_user_published_idx;
ALTER TABLE posts DROP COLUMN quality_rules;
ALTER TABLE posts DROP COLUMN replacement_candidates;
ALTER TABLE posts DROP COLUMN content_nouns;
ALTER TABLE posts DROP COLUMN field;
-- published_at owns the CHECK that names published_url, and SQLite refuses to drop a column
-- another column's CHECK names, so it goes first.
ALTER TABLE posts DROP COLUMN published_at;
ALTER TABLE posts DROP COLUMN published_url;
