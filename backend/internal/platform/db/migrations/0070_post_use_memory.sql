-- +goose Up
-- The post's memory opt-in (MEM-18). A generation option like target_length and tag_count:
-- it decides what a RUN may carry, not what the post is, so it changes no status, revision,
-- baseline or learning eligibility.
--
-- One added column with a NOT NULL default, so goose's own transaction is enough and no
-- table is rebuilt. The default is what makes MEM-1's promise hold for every row that
-- already exists: off, and a post with the option off builds a prompt byte-identical to the
-- one it built before this domain existed.
ALTER TABLE posts ADD COLUMN use_memory INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE posts DROP COLUMN use_memory;
