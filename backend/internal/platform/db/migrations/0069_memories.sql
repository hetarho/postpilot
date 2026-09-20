-- +goose Up
-- Memories (MEM r1). A memory is ONE atomic fact about the author's world, authored by the
-- user approving an extracted candidate or by writing it by hand. Nothing here is learned:
-- no model creates, approves, edits, ranks, retires or deletes a row (MEM-23), and no row
-- reaches a prompt except through a post that opted in (MEM-19).
--
-- Three new tables, so goose's own transaction is enough: no table is rebuilt and no
-- composite FK is added to an existing parent, unlike 0009/0011/0022.
--
-- No seed rows and no backfill. An existing account starts with no memories, and MEM-1's
-- promise is exactly that: with none, every prompt stays byte-identical to today's.
CREATE TABLE memories (
    id      TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text    TEXT NOT NULL,
    -- The closed five (MEM-5). A CHECK rather than a lookup table: the set is code, the
    -- retrieval halves rest on it, and there are no user-defined kinds to store.
    kind    TEXT NOT NULL CHECK (kind IN ('preference','persona','place','person','history')),
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    -- Advanced when the same text is approved again (MEM-9). Separate from updated_at,
    -- which belongs to an authored edit: retrieval breaks ties by use, not by edit.
    last_seen_at TEXT NOT NULL,
    -- Exact-text dedup per account as a constraint rather than a service check, so two
    -- concurrent approvals cannot both insert. The same reasoning guidelines(user_id, text)
    -- already carries. Exact after trim and nothing else — no similarity anywhere (MEM-9).
    UNIQUE (user_id, text),
    -- The composite target the two child tables point at, so a child row carries the owner
    -- and a cross-account link is impossible rather than merely unwritten.
    UNIQUE (id, user_id)
);

CREATE TABLE memory_tags (
    memory_id TEXT NOT NULL,
    user_id   TEXT NOT NULL,
    tag       TEXT NOT NULL,
    PRIMARY KEY (memory_id, tag),
    FOREIGN KEY (memory_id, user_id) REFERENCES memories(id, user_id) ON DELETE CASCADE
);

-- Retrieval's entire reason for a tag table rather than a JSON column: the selection reads
-- the account's tags and matches them against one post's key (MEM-7).
CREATE INDEX idx_memory_tags_lookup ON memory_tags(user_id, tag);

CREATE TABLE memory_sources (
    memory_id TEXT NOT NULL,
    -- The post's SLUG, never a post row id: the slug is minted once and never changes.
    --
    -- Deliberately a plain column with NO foreign key to posts, the way
    -- guideline_candidates.post_slug is. MEM-17 deletes a memory only when the deleted
    -- post held its LAST link, and that is a question about the links that existed a
    -- moment ago: an ON DELETE CASCADE would have taken them away before the post
    -- context's hook could count them, leaving a hand-written memory (which has no links
    -- at all) indistinguishable from an orphaned one.
    post_slug TEXT NOT NULL,
    user_id   TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (memory_id, post_slug),
    FOREIGN KEY (memory_id, user_id) REFERENCES memories(id, user_id) ON DELETE CASCADE
);

-- Serves the post-delete hook: every link one deleted post held, for one account.
CREATE INDEX idx_memory_sources_post ON memory_sources(user_id, post_slug);

-- +goose Down
DROP INDEX IF EXISTS idx_memory_sources_post;
DROP TABLE IF EXISTS memory_sources;
DROP INDEX IF EXISTS idx_memory_tags_lookup;
DROP TABLE IF EXISTS memory_tags;
DROP TABLE IF EXISTS memories;
