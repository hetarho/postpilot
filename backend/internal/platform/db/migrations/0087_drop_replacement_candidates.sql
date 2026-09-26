-- +goose Up
-- The spans a write offered field phrases for. Generation no longer freezes phrases and posts no
-- longer carry candidates (GEN-14, GEN-55), and no row ever held one, so the column goes. No
-- index, key, view or other column's CHECK names it; its own json_valid CHECK leaves with it.
ALTER TABLE posts DROP COLUMN replacement_candidates;

-- +goose Down
-- A rolled-back binary reads and writes the column, so it comes back empty, with 0077's CHECK:
-- there is no value to restore, and NULL reads as none in that code.
ALTER TABLE posts ADD COLUMN replacement_candidates TEXT CHECK (replacement_candidates IS NULL OR json_valid(replacement_candidates));
