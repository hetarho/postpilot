-- +goose Up
-- A post's campaign (sponsored review) switch, advertiser guide and pre-submission checklist ticks (POST-117,
-- POST-118, POST-119). Every post written before the switch existed reads as off; the create
-- path inserts 1, so a new post starts as a campaign post. The guide's lists and the ticks are
-- JSON arrays of strings; an absent minimum is NULL.
ALTER TABLE posts ADD COLUMN campaign INTEGER NOT NULL DEFAULT 0 CHECK (campaign IN (0, 1));
ALTER TABLE posts ADD COLUMN advertiser_guide TEXT NOT NULL DEFAULT '';
ALTER TABLE posts ADD COLUMN advertiser_keywords TEXT NOT NULL DEFAULT '[]';
ALTER TABLE posts ADD COLUMN advertiser_min_chars INTEGER;
ALTER TABLE posts ADD COLUMN advertiser_min_photos INTEGER;
ALTER TABLE posts ADD COLUMN advertiser_hashtags TEXT NOT NULL DEFAULT '[]';
ALTER TABLE posts ADD COLUMN checklist_ticks TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE posts DROP COLUMN checklist_ticks;
ALTER TABLE posts DROP COLUMN advertiser_hashtags;
ALTER TABLE posts DROP COLUMN advertiser_min_photos;
ALTER TABLE posts DROP COLUMN advertiser_min_chars;
ALTER TABLE posts DROP COLUMN advertiser_keywords;
ALTER TABLE posts DROP COLUMN advertiser_guide;
ALTER TABLE posts DROP COLUMN campaign;
