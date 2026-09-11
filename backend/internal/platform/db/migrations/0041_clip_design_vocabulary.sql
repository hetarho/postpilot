-- +goose Up
-- The CDS plan vocabulary (CDS-12, CDS-22): three copy styles become four and the
-- three caption positions become the four vertical anchors plus an alignment.
--
-- Both columns hold `encoding/json` output of Go structs with no field tags, so the
-- stored keys are the Go field names — "Style", "Position" — and the encoder emits no
-- whitespace. Each token below therefore carries its own quotes, key and colon, and a
-- caption's own text can never match one: a quote inside text is escaped as \".
-- `clip.migrateStoredPlan` applies exactly these tokens on read, so a plan written by
-- the old binary during the deploy window cannot fail; this migration is the authority.
--
-- "center" becomes LOWER_MID rather than UPPER_MID: the old centre sat at y ≈ 846 in
-- 9:16, between UPPER_MID 700 and LOWER_MID 1100, and LOWER_MID keeps a bottom-leaning
-- caption closest to where the owner approved it.
UPDATE clip_projects SET edit_plan_json = REPLACE(
    REPLACE(
        REPLACE(
            REPLACE(edit_plan_json, '"Style":"diary"', '"Style":"memo"'),
            '"Style":"emphasis"', '"Style":"bold"'),
        '"Position":"center"', '"Position":"lower_mid"'),
    -- Renames the key and gives every caption the alignment the old vocabulary
    -- had no field for; CENTER is what all three former positions rendered as.
    '"Position":"', '"Align":"center","Anchor":"')
WHERE edit_plan_json IS NOT NULL;

-- The plan also freezes the template's approved set, as a bare JSON array whose
-- elements carry no key. Those tokens hold the structural character on BOTH sides:
-- a bare '"diary"' would also match a caption whose text is the word diary.
UPDATE clip_projects SET edit_plan_json = REPLACE(REPLACE(REPLACE(REPLACE(
    REPLACE(REPLACE(REPLACE(REPLACE(edit_plan_json,
        '["diary",', '["memo",'), ',"diary",', ',"memo",'),
        ',"diary"]', ',"memo"]'), '["diary"]', '["memo"]'),
        '["emphasis",', '["bold",'), ',"emphasis",', ',"bold",'),
        ',"emphasis"]', ',"bold"]'), '["emphasis"]', '["bold"]')
WHERE edit_plan_json IS NOT NULL;

-- The column itself holds nothing but the array, so the element tokens need no
-- delimiters here.
UPDATE video_templates
SET copy_styles = REPLACE(REPLACE(copy_styles, '"diary"', '"memo"'), '"emphasis"', '"bold"');

-- Every CDS fallback lands on 깔끔하게, so an approved set without it cannot render.
UPDATE video_templates
SET copy_styles = CASE WHEN copy_styles = '[]' THEN '["clean"]'
                       ELSE '["clean",' || substr(copy_styles, 2) END
WHERE copy_styles NOT LIKE '%"clean"%';

-- +goose Down
UPDATE clip_projects SET edit_plan_json = REPLACE(
    REPLACE(
        REPLACE(
            REPLACE(edit_plan_json, '"Align":"center","Anchor":"', '"Position":"'),
            '"Position":"lower_mid"', '"Position":"center"'),
        '"Style":"bold"', '"Style":"emphasis"'),
    '"Style":"memo"', '"Style":"diary"')
WHERE edit_plan_json IS NOT NULL;

-- The two anchors the old vocabulary had no value for fall back to its centre.
UPDATE clip_projects SET edit_plan_json =
    REPLACE(REPLACE(edit_plan_json, '"Position":"upper_mid"', '"Position":"center"'),
            '"Position":"lower_mid"', '"Position":"center"')
WHERE edit_plan_json IS NOT NULL;

UPDATE clip_projects SET edit_plan_json = REPLACE(REPLACE(REPLACE(REPLACE(
    REPLACE(REPLACE(REPLACE(REPLACE(edit_plan_json,
        '["memo",', '["diary",'), ',"memo",', ',"diary",'),
        ',"memo"]', ',"diary"]'), '["memo"]', '["diary"]'),
        '["bold",', '["emphasis",'), ',"bold",', ',"emphasis",'),
        ',"bold"]', ',"emphasis"]'), '["bold"]', '["emphasis"]')
WHERE edit_plan_json IS NOT NULL;

UPDATE video_templates
SET copy_styles = REPLACE(REPLACE(copy_styles, '"memo"', '"diary"'), '"bold"', '"emphasis"');

-- 형광펜 has no pre-CDS equivalent; a template left with only it keeps 깔끔하게.
UPDATE video_templates SET copy_styles = REPLACE(
    REPLACE(REPLACE(copy_styles, '["mark",', '["'), ',"mark"]', '"]'), '["mark"]', '["clean"]')
WHERE copy_styles LIKE '%"mark"%';
