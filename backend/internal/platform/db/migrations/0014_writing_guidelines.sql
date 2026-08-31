-- +goose Up
-- Writing guidelines (plan 16). A guideline is one reusable account-owned rule about what a
-- post must avoid or watch out for, applied to every generation of the account or scoped to
-- specific purposes. Voice keeps deciding how sentences sound and the purpose keeps deciding
-- genre; a guideline is a prohibition or caution, not a brief.
--
-- Both tables are new, so no table is rebuilt and goose's transaction is enough here — the
-- NO TRANSACTION dance 0009/0011 needed was about adding a composite FK to an existing
-- parent table.
--
-- No seed rows and no backfill: existing accounts start with zero guidelines, which is what
-- keeps every prompt built today byte-identical to the one built after this migration.
CREATE TABLE guidelines (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('global','purposes')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    -- The composite target guideline_purposes points at, so a link carries the account
    -- through the FK itself.
    UNIQUE (id, user_id),
    -- Exact-text dedup per account is a constraint rather than a service check, so two
    -- concurrent creates of the same rule cannot both win.
    UNIQUE (user_id, text)
);

-- The scope set for a 'purposes' guideline. user_id is carried on the row so BOTH foreign
-- keys can be composite: a guideline can then never link another account's purpose even if
-- a service check is bypassed, which is the same protection posts.purpose_id has.
--
-- ON DELETE CASCADE on both sides deletes only link rows. Deleting a purpose therefore
-- unlinks it from every guideline in the same statement while the guideline rows survive;
-- one left with no links applies nowhere until it is rescoped. (The SET NULL trap that
-- forced plan 11's trigger does not apply: here the child row itself is what goes away.)
CREATE TABLE guideline_purposes (
    guideline_id TEXT NOT NULL,
    purpose_id   TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    PRIMARY KEY (guideline_id, purpose_id),
    FOREIGN KEY (guideline_id, user_id) REFERENCES guidelines(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (purpose_id, user_id) REFERENCES purposes(id, user_id) ON DELETE CASCADE
);

-- Scope resolution reads the links of one purpose for one account at enqueue time.
CREATE INDEX idx_guideline_purposes_purpose ON guideline_purposes(user_id, purpose_id);

-- The list and the global-group resolution both read one account ordered by creation time.
CREATE INDEX idx_guidelines_user_created ON guidelines(user_id, created_at, id);

-- +goose Down
DROP INDEX idx_guidelines_user_created;
DROP INDEX idx_guideline_purposes_purpose;
DROP TABLE guideline_purposes;
DROP TABLE guidelines;
