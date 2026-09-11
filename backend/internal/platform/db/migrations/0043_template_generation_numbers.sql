-- +goose Up
-- A template's two generation numbers (TEMPLATE-47).
--
-- A template decides the SHAPE of a post, and how long that shape usually runs and how many
-- tags it wants are part of the shape — a 정보성 리뷰 is not the same length as a 짧은 소식.
-- They are SEEDS and nothing else: assigning the template copies a set one onto the post's
-- own option (TEMPLATE-48), and no prompt, payload, freeze or snapshot ever reads them here.
--
-- NULLABLE AND NOT BACKFILLED. NULL is 의견 없음 — the template says nothing about that
-- number and assigning it leaves the post's own value alone. Every template written before
-- this migration means exactly that, so there is nothing to fill in.
--
-- No CHECK on either: the bounds are the POST option's (a positive length, a tag count in
-- POST_TAG_COUNT_MIN…MAX) and they live in one place, the template service. A second copy
-- in SQL would drift the day the product moves one of them.
ALTER TABLE templates ADD COLUMN target_length INTEGER;
ALTER TABLE templates ADD COLUMN tag_count INTEGER;

-- +goose Down
ALTER TABLE templates DROP COLUMN target_length;
ALTER TABLE templates DROP COLUMN tag_count;
