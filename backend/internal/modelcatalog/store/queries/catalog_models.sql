-- ASCII only: sqlc slices these statements by byte offset but counts in runes, so a
-- multi-byte character anywhere above rotates every generated query text.
--
-- The curated model catalog. The table is global, not per-account: what an installation
-- offers is an operator decision.
--
-- Two groups of columns with different owners. The curation columns (reasoning_effort,
-- enabled) are only ever written by an operator edit. The snapshot
-- columns (label, flags, context, pricing) and the availability columns (listed,
-- last_seen_at) are only ever written from a successful upstream read. updated_at tracks
-- the first group alone, so a refresh does not make every row look freshly curated.

-- name: ListCatalogModels :many
SELECT model_id, provider_slug, label, vision, structured_output,
       context_tokens, input_usd_per_million, output_usd_per_million, pricing_checked_at,
       reasoning_effort, enabled, listed, last_seen_at, created_at, updated_at
FROM catalog_models
ORDER BY provider_slug, model_id;

-- name: GetCatalogModel :one
SELECT model_id, provider_slug, label, vision, structured_output,
       context_tokens, input_usd_per_million, output_usd_per_million, pricing_checked_at,
       reasoning_effort, enabled, listed, last_seen_at, created_at, updated_at
FROM catalog_models
WHERE model_id = ?;

-- name: UpsertCatalogModel :exec
INSERT INTO catalog_models (
    model_id, provider_slug, label, vision, structured_output,
    context_tokens, input_usd_per_million, output_usd_per_million, pricing_checked_at,
    reasoning_effort, enabled, listed, last_seen_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(model_id) DO UPDATE SET
    provider_slug = excluded.provider_slug,
    label = excluded.label,
    vision = excluded.vision,
    structured_output = excluded.structured_output,
    context_tokens = excluded.context_tokens,
    input_usd_per_million = excluded.input_usd_per_million,
    output_usd_per_million = excluded.output_usd_per_million,
    pricing_checked_at = excluded.pricing_checked_at,
    reasoning_effort = excluded.reasoning_effort,
    enabled = excluded.enabled,
    listed = excluded.listed,
    last_seen_at = excluded.last_seen_at,
    updated_at = excluded.updated_at;
-- created_at is absent from the DO UPDATE list on purpose: the row keeps the moment it
-- entered the catalog, so only the insert may set it.

-- name: UpdateCatalogModelCuration :execrows
UPDATE catalog_models
SET enabled = ?, reasoning_effort = ?, updated_at = ?
WHERE model_id = ?;

-- Availability, step 1: assume nothing is offered any more. Step 2 puts back everything the
-- provider actually listed, in the same transaction, so no reader sees the gap. updated_at
-- is untouched: an availability sweep is not a curation edit.
-- name: UnlistAllCatalogModels :exec
UPDATE catalog_models SET listed = 0;

-- name: MarkCatalogModelSeen :exec
UPDATE catalog_models
SET provider_slug = ?, label = ?, vision = ?, structured_output = ?,
    context_tokens = ?, input_usd_per_million = ?, output_usd_per_million = ?,
    pricing_checked_at = ?, listed = 1, last_seen_at = ?
WHERE model_id = ?;
