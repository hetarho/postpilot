-- +goose Up
-- CLIP-178: the storyline a 바로 만들기 flow call opens with — its paragraphs in order, each naming
-- the observed scenes it uses, whether the owner edited it by hand, and the sources it was
-- written from. NULL is none: every clip written before it.
ALTER TABLE clip_projects ADD COLUMN storyline_json TEXT CHECK (storyline_json IS NULL OR json_valid(storyline_json));

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN storyline_json;
