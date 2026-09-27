-- +goose Up
-- The category-preset clip path is retired (CLIP-4, CLIP-14, CLIP-68, CDS-37, CDS-51): a
-- template is an outline body under a name, and no preset, fact chip, opening hook, closing
-- call to action or label-keyed answer exists. Before closed alpha the stored data is
-- disposable, so nothing legacy is carried forward: a template that is not an outline goes,
-- a snapshot synthesized from a recipe goes, the recipe columns, the call to action and the
-- answers go, and every stored plan loses the keys of fields the code no longer has.
--
-- clip_finalized_content names cta, and SQLite cannot drop a column a trigger names. It
-- would also refuse the snapshot and plan rewrites below on a finalized project, whose plan
-- has to stay readable too. It is dropped first and recreated without cta at the end.
DROP TRIGGER clip_finalized_content;

-- A template with no outline body, or one whose body the server synthesized from a
-- category-preset recipe. video_templates_detach clears it from every project that used it.
DELETE FROM video_templates
WHERE composition_body IS NULL OR trim(composition_body) = '' OR composition_legacy <> 0;

-- A snapshot synthesized from such a recipe is dropped with its inputs, so the project reads
-- as what it now is: a project with no template. Every other snapshot loses the two keys.
UPDATE clip_projects SET composition_snapshot_json = NULL, composition_inputs_json = NULL
WHERE composition_snapshot_json IS NOT NULL
  AND (NOT json_valid(composition_snapshot_json)
       OR json_extract(composition_snapshot_json, '$.legacy') = 1
       OR json_type(composition_snapshot_json, '$.legacy_recipe') IS NOT NULL);
UPDATE clip_projects SET composition_snapshot_json = json_remove(composition_snapshot_json, '$.legacy', '$.legacy_recipe')
WHERE composition_snapshot_json IS NOT NULL;

-- Stored plans are `encoding/json` output of structs with no field tags, so each key is a Go
-- field name: the legacy opening sentence, the native-editing marker only a legacy snapshot
-- set, the two legacy snapshot keys, and each cut's fact chips, on the kept cuts and on the
-- retired ones a session undo may restore. The same paths hold in every stored envelope
-- version. Rebuilt arrays keep their order.
UPDATE clip_projects SET edit_plan_json = json_remove(edit_plan_json,
    '$.Plan.Hook', '$.Composition.NativeEditing', '$.Composition.Snapshot.Legacy', '$.Composition.Snapshot.LegacyRecipe')
WHERE edit_plan_json IS NOT NULL AND json_valid(edit_plan_json);
UPDATE clip_projects SET edit_plan_json = json_set(edit_plan_json, '$.Plan.Cuts', json((
    SELECT json_group_array(json(cut)) FROM (
        SELECT json_remove(value, '$.Chips') AS cut FROM json_each(clip_projects.edit_plan_json, '$.Plan.Cuts') ORDER BY key))))
WHERE edit_plan_json IS NOT NULL AND json_valid(edit_plan_json) AND json_type(edit_plan_json, '$.Plan.Cuts') = 'array';
UPDATE clip_projects SET edit_plan_json = json_set(edit_plan_json, '$.Composition.RetiredCuts', json((
    SELECT json_group_array(json(cut)) FROM (
        SELECT json_remove(value, '$.Chips') AS cut FROM json_each(clip_projects.edit_plan_json, '$.Composition.RetiredCuts') ORDER BY key))))
WHERE edit_plan_json IS NOT NULL AND json_valid(edit_plan_json) AND json_type(edit_plan_json, '$.Composition.RetiredCuts') = 'array';

-- A staged attempt result waits for the finisher with the same envelope.
UPDATE clip_attempt_results SET edit_plan_json = json_remove(edit_plan_json,
    '$.Plan.Hook', '$.Composition.NativeEditing', '$.Composition.Snapshot.Legacy', '$.Composition.Snapshot.LegacyRecipe')
WHERE edit_plan_json <> '' AND json_valid(edit_plan_json);
UPDATE clip_attempt_results SET edit_plan_json = json_set(edit_plan_json, '$.Plan.Cuts', json((
    SELECT json_group_array(json(cut)) FROM (
        SELECT json_remove(value, '$.Chips') AS cut FROM json_each(clip_attempt_results.edit_plan_json, '$.Plan.Cuts') ORDER BY key))))
WHERE edit_plan_json <> '' AND json_valid(edit_plan_json) AND json_type(edit_plan_json, '$.Plan.Cuts') = 'array';
UPDATE clip_attempt_results SET edit_plan_json = json_set(edit_plan_json, '$.Composition.RetiredCuts', json((
    SELECT json_group_array(json(cut)) FROM (
        SELECT json_remove(value, '$.Chips') AS cut FROM json_each(clip_attempt_results.edit_plan_json, '$.Composition.RetiredCuts') ORDER BY key))))
WHERE edit_plan_json <> '' AND json_valid(edit_plan_json) AND json_type(edit_plan_json, '$.Composition.RetiredCuts') = 'array';

-- The recipe columns. Each CHECK that names one is its own and leaves with it; no index,
-- view or trigger names any of them.
ALTER TABLE video_templates DROP COLUMN information_fields;
ALTER TABLE video_templates DROP COLUMN cut_guidance;
ALTER TABLE video_templates DROP COLUMN copy_styles;
ALTER TABLE video_templates DROP COLUMN accent;
ALTER TABLE video_templates DROP COLUMN preset;
ALTER TABLE video_templates DROP COLUMN caption_pace;
ALTER TABLE video_templates DROP COLUMN composition_legacy;
ALTER TABLE clip_projects DROP COLUMN cta;
-- Its four triggers leave with the table.
DROP TABLE clip_project_answers;

-- +goose StatementBegin
CREATE TRIGGER clip_finalized_content BEFORE UPDATE OF title,ratio,target_duration_ms,disclosure,hide_disclosure,analysis_json,edit_plan_json,edit_plan_revision,rendered_plan_revision,result_key,result_id,result_content_type,result_bytes,result_duration_ms,result_created_at,composition_snapshot_json,composition_inputs_json ON clip_projects
WHEN OLD.finalized_at IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd

-- +goose Down
-- A rolled-back binary reads and writes every retired column and the answers table, so they
-- come back empty with their original DDL: there is no value to restore, a template reads as
-- an outline (composition_legacy 0) with no recipe, and a project as one with no call to
-- action and no answer. Deleted templates and rewritten plans stay as they are.
DROP TRIGGER clip_finalized_content;
ALTER TABLE video_templates ADD COLUMN information_fields TEXT NOT NULL DEFAULT '[]';
ALTER TABLE video_templates ADD COLUMN cut_guidance TEXT NOT NULL DEFAULT '';
ALTER TABLE video_templates ADD COLUMN copy_styles TEXT NOT NULL DEFAULT '[]';
ALTER TABLE video_templates ADD COLUMN accent TEXT;
ALTER TABLE video_templates ADD COLUMN preset TEXT NOT NULL DEFAULT '';
ALTER TABLE video_templates ADD COLUMN caption_pace TEXT NOT NULL DEFAULT '' CHECK (caption_pace IN ('', 'steady', 'rapid'));
ALTER TABLE video_templates ADD COLUMN composition_legacy INTEGER NOT NULL DEFAULT 0 CHECK (composition_legacy IN (0,1));
ALTER TABLE clip_projects ADD COLUMN cta TEXT NOT NULL DEFAULT '';
CREATE TABLE clip_project_answers (
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    answer TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(project_id, label),
    FOREIGN KEY(project_id, user_id) REFERENCES clip_projects(id, user_id) ON DELETE CASCADE
);
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_answers_insert BEFORE INSERT ON clip_project_answers
WHEN EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_answers_update BEFORE UPDATE ON clip_project_answers
WHEN EXISTS(SELECT 1 FROM clip_projects WHERE id=OLD.project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_answers_busy_insert BEFORE INSERT ON clip_project_answers
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=NEW.project_id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_answers_busy_update BEFORE UPDATE ON clip_project_answers
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.project_id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_content BEFORE UPDATE OF title,ratio,target_duration_ms,disclosure,cta,hide_disclosure,analysis_json,edit_plan_json,edit_plan_revision,rendered_plan_revision,result_key,result_id,result_content_type,result_bytes,result_duration_ms,result_created_at,composition_snapshot_json,composition_inputs_json ON clip_projects
WHEN OLD.finalized_at IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
