-- +goose Up
-- Earlier clips read as off; the application explicitly inserts the new-project default.
ALTER TABLE clip_projects ADD COLUMN campaign INTEGER NOT NULL DEFAULT 0 CHECK(campaign IN (0,1));
ALTER TABLE clip_projects ADD COLUMN advertiser_guide TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_projects ADD COLUMN advertiser_keywords TEXT NOT NULL DEFAULT '[]';
ALTER TABLE clip_projects ADD COLUMN advertiser_hashtags TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN advertiser_hashtags;
ALTER TABLE clip_projects DROP COLUMN advertiser_keywords;
ALTER TABLE clip_projects DROP COLUMN advertiser_guide;
ALTER TABLE clip_projects DROP COLUMN campaign;
