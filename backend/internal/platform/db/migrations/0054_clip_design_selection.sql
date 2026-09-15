-- +goose Up
-- Preserve authored XML bytes; never parse/reserialize or restructure old skeletons.
-- Track quotes so an attribute-looking literal is never treated as markup.
WITH RECURSIVE scan(id, body, pos, output, tag, quote, has_intro, insert_after) AS (
    SELECT id, composition_body, 1, '', '', '', 0, 0
    FROM video_templates WHERE composition_body IS NOT NULL
    UNION ALL
    SELECT id, body,
        pos + CASE WHEN (CASE WHEN quote = '' AND tag = 'clip' AND substr(body, pos, 9) = ' styles="' THEN 9 + instr(substr(body, pos + 9), '"')
            WHEN quote = '' AND tag = 'text' AND substr(body, pos, 8) = ' style="' THEN 8 + instr(substr(body, pos + 8), '"') ELSE 0 END) > 0 THEN (CASE WHEN quote = '' AND tag = 'clip' AND substr(body, pos, 9) = ' styles="' THEN 9 + instr(substr(body, pos + 9), '"')
            WHEN quote = '' AND tag = 'text' AND substr(body, pos, 8) = ' style="' THEN 8 + instr(substr(body, pos + 8), '"') ELSE 0 END) ELSE 1 END,
        output || CASE WHEN (CASE WHEN quote = '' AND tag = 'clip' AND substr(body, pos, 9) = ' styles="' THEN 9 + instr(substr(body, pos + 9), '"')
            WHEN quote = '' AND tag = 'text' AND substr(body, pos, 8) = ' style="' THEN 8 + instr(substr(body, pos + 8), '"') ELSE 0 END) > 0 THEN '' ELSE substr(body, pos, 1) END,
        CASE WHEN quote = '' AND substr(body, pos, 1) = '<' THEN
            CASE WHEN substr(body, pos, 5) = '<clip' THEN 'clip'
                 WHEN substr(body, pos, 5) = '<text' THEN 'text' ELSE 'other' END
             WHEN quote = '' AND substr(body, pos, 1) = '>' THEN '' ELSE tag END,
        CASE WHEN (CASE WHEN quote = '' AND tag = 'clip' AND substr(body, pos, 9) = ' styles="' THEN 9 + instr(substr(body, pos + 9), '"')
            WHEN quote = '' AND tag = 'text' AND substr(body, pos, 8) = ' style="' THEN 8 + instr(substr(body, pos + 8), '"') ELSE 0 END) > 0 THEN quote
             WHEN tag <> '' AND quote = '' AND substr(body, pos, 1) IN ('"', char(39)) THEN substr(body, pos, 1)
             WHEN quote <> '' AND substr(body, pos, 1) = quote THEN '' ELSE quote END,
        CASE WHEN tag = 'clip' AND quote = '' AND substr(body, pos, 6) = 'intro=' THEN 1 ELSE has_intro END,
        CASE WHEN tag = 'clip' AND quote = '' AND substr(body, pos, 11) = 'version="1"' THEN length(output) + 11 ELSE insert_after END
    FROM scan WHERE pos <= length(body)
), converted AS (
    SELECT id, CASE WHEN has_intro = 0 AND insert_after > 0 THEN
        substr(output, 1, insert_after) || ' intro="b" caption="bold" outro="e"' || substr(output, insert_after + 1)
        ELSE output END AS body
    FROM scan WHERE pos > length(body)
)
UPDATE video_templates SET composition_body = (SELECT body FROM converted WHERE converted.id = video_templates.id)
WHERE composition_body IS NOT NULL;

UPDATE video_templates SET copy_styles = '[]';
UPDATE clip_projects SET edit_plan_json = json_remove(edit_plan_json, '$.CopyStyles', '$.Styles')
WHERE edit_plan_json IS NOT NULL AND json_valid(edit_plan_json)
  AND (json_type(edit_plan_json, '$.CopyStyles') IS NOT NULL OR json_type(edit_plan_json, '$.Styles') IS NOT NULL);

-- +goose Down
-- Data-only retirement: old style permissions cannot be reconstructed.
SELECT 1;
