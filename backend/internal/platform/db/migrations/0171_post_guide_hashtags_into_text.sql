-- +goose Up
-- POST r57 (T912): a post's advertiser guide keeps its hashtags in the free text (POST-118), so
-- the separate list goes. Each stored hashtag the text does not already hold as a #tag token is
-- appended to the text, space-separated on a new last line, so none is lost; a post whose list
-- is empty or already held in its text keeps its text byte-for-byte. Stored hashtags carry no
-- leading # (the guide normalizer strips it). The text is not truncated here: a long guide may
-- pass the save limit, which the owner's next edit of the text meets. A clip's guide keeps its
-- own list (CLIP-210).
UPDATE posts SET advertiser_guide = advertiser_guide
    || CASE WHEN advertiser_guide = '' THEN '' ELSE char(10) END
    || (SELECT group_concat('#' || tag.value, ' ') FROM json_each(posts.advertiser_hashtags) AS tag
        WHERE instr(posts.advertiser_guide, '#' || tag.value) = 0)
WHERE EXISTS (SELECT 1 FROM json_each(posts.advertiser_hashtags) AS tag
    WHERE instr(posts.advertiser_guide, '#' || tag.value) = 0);
ALTER TABLE posts DROP COLUMN advertiser_hashtags;

-- +goose Down
-- The column returns empty; the hashtags folded into the text stay there.
ALTER TABLE posts ADD COLUMN advertiser_hashtags TEXT NOT NULL DEFAULT '[]';
