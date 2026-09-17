-- +goose Up
-- The design selection belongs to the PROJECT (CLIP-139): the intro preset, the
-- outro preset and the caption styles this clip may use are chosen in ①, and the
-- template only supplies their starting values (CLIP-14). They used to live in
-- the composition body's root attributes, which made the template a precondition
-- for a choice that was never the template's to keep.
ALTER TABLE clip_projects ADD COLUMN intro_preset TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_projects ADD COLUMN outro_preset TEXT NOT NULL DEFAULT '';
-- A JSON array of style ids, read whole at layout and never queried across
-- projects (CDS-80). '[]' resolves to the default style alone, so a row this
-- backfill cannot read renders exactly as it does today.
ALTER TABLE clip_projects ADD COLUMN allowed_caption_styles TEXT NOT NULL DEFAULT '[]';

-- Every existing project keeps rendering what it renders now (CLIP-144): the
-- selection is COPIED off the document the render already reads — the frozen
-- snapshot where a generation made one, else the attached template's body — and
-- only a row with neither takes the defaults. The attribute is read from the
-- root `<clip …>` tag alone, never from the body at large, and any value the
-- grammar would refuse falls back to the default rather than being invented.
WITH document AS (
    SELECT p.id AS id,
        COALESCE(
            CASE WHEN p.composition_snapshot_json IS NOT NULL AND json_valid(p.composition_snapshot_json)
                 THEN json_extract(p.composition_snapshot_json, '$.body') END,
            t.composition_body, '') AS body
    FROM clip_projects p
    LEFT JOIN video_templates t ON t.id = p.video_template_id AND t.user_id = p.user_id
), root AS (
    SELECT id, CASE WHEN instr(body, '>') > 0 THEN substr(body, 1, instr(body, '>')) ELSE '' END AS tag
    FROM document
), selection AS (
    SELECT id,
        CASE WHEN instr(tag, ' intro="') > 0
             THEN substr(tag, instr(tag, ' intro="') + 8, instr(substr(tag, instr(tag, ' intro="') + 8), '"') - 1)
             ELSE '' END AS intro,
        CASE WHEN instr(tag, ' outro="') > 0
             THEN substr(tag, instr(tag, ' outro="') + 8, instr(substr(tag, instr(tag, ' outro="') + 8), '"') - 1)
             ELSE '' END AS outro
    FROM root
)
UPDATE clip_projects SET
    intro_preset = (SELECT CASE WHEN intro IN ('a', 'b') THEN intro ELSE 'b' END FROM selection WHERE selection.id = clip_projects.id),
    outro_preset = (SELECT CASE WHEN outro IN ('b', 'e') THEN outro ELSE 'e' END FROM selection WHERE selection.id = clip_projects.id),
    -- The grammar admitted exactly one caption treatment, so the copy is that
    -- one style for every project that already exists.
    allowed_caption_styles = '["bold"]';

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN intro_preset;
ALTER TABLE clip_projects DROP COLUMN outro_preset;
ALTER TABLE clip_projects DROP COLUMN allowed_caption_styles;
