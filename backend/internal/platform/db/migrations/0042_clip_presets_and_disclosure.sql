-- +goose Up
-- A video template's category preset (CDS-50) and a clip's disclosure and CTA
-- (CDS-31, CDS-29).
--
-- All three are NOT NULL DEFAULT '' rather than nullable: the empty string is a
-- real, readable state — a template written before presets existed, and a clip
-- whose owner has not yet chosen its campaign type — and the domain gives each
-- one its own meaning. Empty disclosure is what the generation gate refuses; an
-- empty CTA falls back to the preset's; an empty preset behaves as the shared
-- defaults of CDS-51. Backfilling a guess would put a category on templates
-- their owner never categorised and a campaign type on clips nobody declared.
--
-- The phrases themselves are code-owned and never stored: these columns hold the
-- fixed ids (ad | sponsored | provided | paid | self and blog | place | save),
-- so the renderer cannot be handed an edited disclosure (CDS-31).
ALTER TABLE video_templates ADD COLUMN preset TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_projects ADD COLUMN disclosure TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_projects ADD COLUMN cta TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE video_templates DROP COLUMN preset;
ALTER TABLE clip_projects DROP COLUMN disclosure;
ALTER TABLE clip_projects DROP COLUMN cta;
