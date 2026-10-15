-- +goose Up
-- Earlier clips retain their ratio and belong to the Naver channel (CLIP-209).
ALTER TABLE clip_projects ADD COLUMN channel TEXT NOT NULL DEFAULT 'naver_clip' CHECK(channel IN ('naver_clip','instagram_clip'));

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN channel;
