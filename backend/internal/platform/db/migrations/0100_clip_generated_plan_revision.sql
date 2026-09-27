-- +goose Up
-- CLIP-180: the plan revision a generation or a revision last wrote. An owner edit advances
-- edit_plan_revision past it, which is how 이 스토리로 만들기 knows it would replace a plan edited
-- by hand. A plan written before it counts as the writer's own.
ALTER TABLE clip_projects ADD COLUMN generated_plan_revision INTEGER NOT NULL DEFAULT 0;
UPDATE clip_projects SET generated_plan_revision = edit_plan_revision WHERE edit_plan_json IS NOT NULL;

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN generated_plan_revision;
