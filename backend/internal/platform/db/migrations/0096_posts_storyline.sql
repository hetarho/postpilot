-- +goose Up
-- GEN-67, POST-99: the storyline the direct write answered first — its paragraphs in order, each
-- naming the attachments it uses, whether the owner edited it by hand, and the attachment names
-- the writing stage was shown. NULL is none: every post written before it, and every draft.
ALTER TABLE posts ADD COLUMN storyline TEXT CHECK (storyline IS NULL OR json_valid(storyline));

-- +goose Down
ALTER TABLE posts DROP COLUMN storyline;
