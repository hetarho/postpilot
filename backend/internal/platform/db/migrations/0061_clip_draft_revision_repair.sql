-- +goose Up
-- Repairs the projects an earlier `TouchCompositionRevision` pushed out of 초안.
--
-- Saving the template's information fields raised `edit_plan_revision` whether or
-- not a plan existed, and CLIP-36 reads any revision above zero as an editing
-- state. So a project whose owner had only filled in ①'s fields opened at
-- ② 클립 다듬기, which has no source picker for fresh footage: the clip could
-- never be generated at all. The query now bumps only where a plan exists, and
-- these rows are put back where that rule would have left them.
--
-- The condition is what "never generated" means in this table: no plan, no
-- rendered revision, and not finalized. Nothing else is touched, and
-- `updated_at` stays as it is — this is a correction, not an edit the owner made.
UPDATE clip_projects
SET edit_plan_revision = 0
WHERE edit_plan_json IS NULL
  AND rendered_plan_revision = 0
  AND finalized_at IS NULL
  AND edit_plan_revision > 0;

-- +goose Down
-- Irreversible on purpose: the revisions removed here counted nothing, and
-- inventing numbers to restore would re-strand the same projects.
SELECT 1;
