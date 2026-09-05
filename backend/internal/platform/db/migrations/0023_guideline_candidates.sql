-- +goose Up
-- Guideline candidates (change 26). A candidate is one completed revision's instruction,
-- recorded verbatim so a correction accrues instead of vanishing with the tab. Nothing here
-- is learned: no model reads, rewrites, generalizes, clusters or ranks a candidate, and no
-- candidate ever reaches a prompt — only an approved guideline does.
--
-- One new table, so goose's own transaction is enough: no table is rebuilt and no composite
-- FK is added to an existing parent, unlike 0009/0011/0022.
--
-- No seed rows and no backfill. There is no revision history to reconstruct, so an existing
-- account starts with an empty candidate list and every prompt built before this migration
-- stays byte-identical to one built after it.
CREATE TABLE guideline_candidates (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    -- The source post. Deliberately a plain nullable column with NO foreign key: deleting
    -- the post must leave the candidate's text intact and only drop the link, and a column
    -- the reader treats as optional is less machinery than an FK with SET NULL.
    post_slug  TEXT,
    -- 'approved' and 'dismissed' rows are kept, not deleted: they are what stops the same
    -- instruction from being recorded again.
    status     TEXT NOT NULL CHECK (status IN ('pending','approved','dismissed')),
    occurrences   INTEGER NOT NULL DEFAULT 1,
    first_seen_at TEXT NOT NULL,
    last_seen_at  TEXT NOT NULL,
    -- Exact-text dedup per account as a constraint rather than a service check, so two
    -- concurrent recordings cannot both insert. The same reasoning guidelines(user_id, text)
    -- already carries.
    UNIQUE (user_id, text)
);

-- Serves the review order exactly: pending rows of one account, most-repeated first.
CREATE INDEX idx_guideline_candidates_review
    ON guideline_candidates(user_id, status, occurrences DESC, last_seen_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_guideline_candidates_review;
DROP TABLE IF EXISTS guideline_candidates;
