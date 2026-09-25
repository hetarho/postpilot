-- +goose Up
-- The most frequent noun M3 once stored beside its share. Nothing has read it since T335, and
-- QUAL-43 decided M3 names no noun, so the column goes. No index, key, CHECK or view names it.
ALTER TABLE post_measurements DROP COLUMN top_noun;

-- +goose Down
-- A rolled-back binary's upsert names the column, so it comes back empty: there is no value to
-- restore, and NULL reads as "" in that code.
ALTER TABLE post_measurements ADD COLUMN top_noun TEXT;
