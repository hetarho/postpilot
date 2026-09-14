-- +goose Up
-- CLIP-18: source audio is off by default and one owner-controlled setting per
-- source decides it. The lease is the owner-facing authority; the edit plan
-- carries a snapshot of it.
ALTER TABLE clip_source_leases ADD COLUMN retain_original_audio INTEGER NOT NULL DEFAULT 0 CHECK(retain_original_audio IN (0,1));

-- CLIP-101: a legacy source keeps the audio meaning its saved plan already had,
-- so rerendering an existing project never silently silences it. Only a plan
-- whose envelope this migration recognizes is read; the stored JSON itself is
-- never rewritten here, because the binary's decoder stays the authority for
-- anything a row-level match cannot settle.
--
-- Version 0 wrote its cuts at the top level with a nullable `Volume` fraction,
-- where a missing value meant full original sound. Versions 1-5 wrote them under
-- `Plan.Cuts` with an integer `VolumePermille`.
UPDATE clip_source_leases SET retain_original_audio=1
WHERE EXISTS (
    SELECT 1 FROM clip_source_batches b
    JOIN clip_projects p ON p.id=b.project_id AND p.user_id=b.user_id
    WHERE b.id=clip_source_leases.batch_id AND b.user_id=clip_source_leases.user_id
      AND p.edit_plan_json IS NOT NULL AND json_valid(p.edit_plan_json)
      AND COALESCE(json_extract(p.edit_plan_json,'$.Version'),0) BETWEEN 0 AND 5
      AND EXISTS (
          SELECT 1 FROM json_each(COALESCE(
              json_extract(p.edit_plan_json,'$.Plan.Cuts'),
              json_extract(p.edit_plan_json,'$.Cuts')
          )) AS cut
          WHERE json_extract(cut.value,'$.SourceID')=clip_source_leases.canonical_id
            AND json_extract(cut.value,'$.Fingerprint')=clip_source_leases.fingerprint
            AND COALESCE(
                json_extract(cut.value,'$.VolumePermille'),
                CASE WHEN json_extract(cut.value,'$.Volume') IS NULL THEN 1000
                     ELSE CAST(json_extract(cut.value,'$.Volume')*1000 AS INTEGER) END
            ) != 0
      )
);

-- A version-6 plan already states the choice explicitly, so it is copied rather
-- than re-derived from per-cut volume, which is an independent gain.
UPDATE clip_source_leases SET retain_original_audio=1
WHERE EXISTS (
    SELECT 1 FROM clip_source_batches b
    JOIN clip_projects p ON p.id=b.project_id AND p.user_id=b.user_id
    WHERE b.id=clip_source_leases.batch_id AND b.user_id=clip_source_leases.user_id
      AND p.edit_plan_json IS NOT NULL AND json_valid(p.edit_plan_json)
      AND json_extract(p.edit_plan_json,'$.Version')=6
      AND EXISTS (
          SELECT 1 FROM json_each(json_extract(p.edit_plan_json,'$.SourceAudio')) AS setting
          WHERE json_extract(setting.value,'$.SourceID')=clip_source_leases.canonical_id
            AND json_extract(setting.value,'$.Fingerprint')=clip_source_leases.fingerprint
            AND json_extract(setting.value,'$.RetainOriginal')=1
      )
);

-- +goose Down
ALTER TABLE clip_source_leases DROP COLUMN retain_original_audio;
