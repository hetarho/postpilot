-- ASCII only: sqlc slices these statements by byte offset but counts in runes, so a
-- multi-byte character anywhere above rotates every generated query text.
--
-- The curated model catalog. The table is global, not per-account: what an installation
-- offers is an operator decision.
--
-- Two groups of columns with different owners. The curation values (the
-- catalog_model_purposes rows and their reasoning_effort) are only ever written by an
-- operator edit. The snapshot
-- columns (label, flags, context, pricing) and the availability columns (listed,
-- last_seen_at) are only ever written from a successful upstream read. updated_at tracks
-- the first group alone, so a refresh does not make every row look freshly curated.

-- name: ListCatalogModels :many
SELECT model_id, provider_slug, label, vision, structured_output, image_output, video_output,
       context_tokens, input_usd_per_million, output_usd_per_million, pricing_checked_at,
       listed, last_seen_at, created_at, updated_at
FROM catalog_models
ORDER BY provider_slug, model_id;

-- name: GetCatalogModel :one
SELECT model_id, provider_slug, label, vision, structured_output, image_output, video_output,
       context_tokens, input_usd_per_million, output_usd_per_million, pricing_checked_at,
       listed, last_seen_at, created_at, updated_at
FROM catalog_models
WHERE model_id = ?;

-- name: ListCatalogModelPurposes :many
SELECT model_id, purpose, reasoning_effort
FROM catalog_model_purposes
ORDER BY model_id, purpose;

-- name: GetCatalogModelPurposes :many
SELECT purpose, reasoning_effort
FROM catalog_model_purposes
WHERE model_id = ?
ORDER BY purpose;

-- name: UpsertCatalogModel :exec
INSERT INTO catalog_models (
    model_id, provider_slug, label, vision, structured_output, image_output, video_output,
    context_tokens, input_usd_per_million, output_usd_per_million, pricing_checked_at,
    listed, last_seen_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(model_id) DO UPDATE SET
    provider_slug = excluded.provider_slug,
    label = excluded.label,
    vision = excluded.vision,
    structured_output = excluded.structured_output,
    image_output = excluded.image_output,
    video_output = excluded.video_output,
    context_tokens = excluded.context_tokens,
    input_usd_per_million = excluded.input_usd_per_million,
    output_usd_per_million = excluded.output_usd_per_million,
    pricing_checked_at = excluded.pricing_checked_at,
    listed = excluded.listed,
    last_seen_at = excluded.last_seen_at,
    updated_at = excluded.updated_at;
-- created_at is absent from the DO UPDATE list on purpose: the row keeps the moment it
-- entered the catalog, so only the insert may set it.

-- The effort is a property of the REGISTRATION, so it is written on the join row and only
-- for a purpose the model actually serves: the WHERE matching zero rows IS the refusal.
-- name: UpdateCatalogModelPurposeReasoning :execrows
UPDATE catalog_model_purposes
SET reasoning_effort = ?
WHERE model_id = ? AND purpose = ?;

-- A deregistration is a curation edit, so it stamps updated_at without touching the
-- curation values themselves.
-- name: TouchCatalogModelCuration :exec
UPDATE catalog_models
SET updated_at = ?
WHERE model_id = ?;

-- Registration is idempotent per (model, purpose): re-checking an already-checked box is
-- not an error, and the join row keeps its own created_at.
-- name: AddCatalogModelPurpose :exec
INSERT OR IGNORE INTO catalog_model_purposes (model_id, purpose, created_at)
VALUES (?, ?, ?);

-- name: RemoveCatalogModelPurpose :exec
DELETE FROM catalog_model_purposes
WHERE model_id = ? AND purpose = ?;

-- Availability, step 1: assume nothing is offered any more. Step 2 puts back everything the
-- provider actually listed, in the same transaction, so no reader sees the gap. updated_at
-- is untouched: an availability sweep is not a curation edit.
-- name: UnlistAllCatalogModels :exec
UPDATE catalog_models SET listed = 0;

-- name: MarkCatalogModelSeen :exec
UPDATE catalog_models
SET provider_slug = ?, label = ?, vision = ?, structured_output = ?,
    image_output = ?, video_output = ?,
    context_tokens = ?, input_usd_per_million = ?, output_usd_per_million = ?,
    pricing_checked_at = ?, listed = 1, last_seen_at = ?
WHERE model_id = ?;
