-- +goose Up
-- Each blog field's phrase list, which only the daily phrase batch wrote. No batch runs, no
-- prompt or screen reads a list and nothing queries the table, so it goes. It carries no
-- user_id, and no index, view, trigger or other table names it.
DROP TABLE field_phrase_lists;

-- +goose Down
-- A rolled-back binary's batch reads and replaces rows here, so the table comes back empty with
-- 0077's DDL: there is no list to restore, and a field with no row is simply due for its first
-- refresh.
CREATE TABLE field_phrase_lists (
    field           TEXT PRIMARY KEY,
    phrases         TEXT NOT NULL CHECK (json_valid(phrases)),
    corpus_size     INTEGER NOT NULL,
    refreshed_at    TEXT,
    next_refresh_at TEXT NOT NULL
);
