-- +goose Up
-- CLIP-75, CLIP-229: 확정 records the plan revision it confirmed; the project reads as
-- confirmed while its plan is still that revision, so a later plan save needs no write here.
-- CLIP-232: a finalized clip may carry its posted URL (게시됨); one without it reads as 완료.
-- CLIP-231: the checklist lines the owner ticked, saved per clip by their text.
ALTER TABLE clip_projects ADD COLUMN confirmed_plan_revision INTEGER CHECK(confirmed_plan_revision IS NULL OR confirmed_plan_revision > 0);
ALTER TABLE clip_projects ADD COLUMN confirmed_at TEXT CHECK((confirmed_at IS NULL) = (confirmed_plan_revision IS NULL));
ALTER TABLE clip_projects ADD COLUMN published_url TEXT CHECK(published_url IS NULL OR (published_url <> '' AND finalized_at IS NOT NULL));
ALTER TABLE clip_projects ADD COLUMN published_at TEXT CHECK((published_at IS NULL) = (published_url IS NULL));
ALTER TABLE clip_projects ADD COLUMN checklist_ticks TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(checklist_ticks) AND json_type(checklist_ticks) = 'array');

-- The confirmation a finalization was made from is frozen with it; the URL and the ticks
-- stay writable, so neither joins clip_finalized_content.
DROP TRIGGER clip_finalized_identity;
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_identity BEFORE UPDATE OF finalized_at,finalized_plan_revision,finalized_result_key,source_access_revoked_at,confirmed_plan_revision,confirmed_at ON clip_projects
WHEN OLD.finalized_at IS NOT NULL AND (NEW.finalized_at IS NOT OLD.finalized_at OR NEW.finalized_plan_revision IS NOT OLD.finalized_plan_revision OR NEW.finalized_result_key IS NOT OLD.finalized_result_key OR NEW.source_access_revoked_at IS NOT OLD.source_access_revoked_at OR NEW.confirmed_plan_revision IS NOT OLD.confirmed_plan_revision OR NEW.confirmed_at IS NOT OLD.confirmed_at)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_finalized_identity;
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_identity BEFORE UPDATE OF finalized_at,finalized_plan_revision,finalized_result_key,source_access_revoked_at ON clip_projects
WHEN OLD.finalized_at IS NOT NULL AND (NEW.finalized_at IS NOT OLD.finalized_at OR NEW.finalized_plan_revision IS NOT OLD.finalized_plan_revision OR NEW.finalized_result_key IS NOT OLD.finalized_result_key OR NEW.source_access_revoked_at IS NOT OLD.source_access_revoked_at)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
ALTER TABLE clip_projects DROP COLUMN checklist_ticks;
ALTER TABLE clip_projects DROP COLUMN published_at;
ALTER TABLE clip_projects DROP COLUMN published_url;
ALTER TABLE clip_projects DROP COLUMN confirmed_at;
ALTER TABLE clip_projects DROP COLUMN confirmed_plan_revision;
