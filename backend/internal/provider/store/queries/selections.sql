-- Queries for the provider context (model_selections). sqlc compiles these into
-- internal/provider/store/sqlc; internal/provider/store maps the rows to domain types.

-- name: UpsertSelection :exec
INSERT INTO model_selections (user_id, stage, slot, provider_id, model_id, updated_at)
VALUES (?, ?, 'active', ?, ?, ?)
ON CONFLICT(user_id, stage, slot) DO UPDATE SET
    provider_id = excluded.provider_id,
    model_id    = excluded.model_id,
    updated_at  = excluded.updated_at;

-- name: ListSelections :many
SELECT user_id, stage, provider_id, model_id, updated_at
FROM model_selections
WHERE user_id = ? AND slot = 'active'
ORDER BY stage;

-- Conditional on the ref: the clear of a vanished choice must not take a choice the
-- user made in the meantime.
-- name: DeleteSelectionIfRef :exec
DELETE FROM model_selections
WHERE user_id = ? AND stage = ? AND slot = ? AND provider_id = ? AND model_id = ?;

-- name: UpsertSelectionSlot :exec
INSERT INTO model_selections (user_id, stage, slot, provider_id, model_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, stage, slot) DO UPDATE SET
    provider_id = excluded.provider_id,
    model_id    = excluded.model_id,
    updated_at  = excluded.updated_at;

-- name: ListSelectionSlots :many
SELECT user_id, stage, slot, provider_id, model_id, updated_at
FROM model_selections
WHERE user_id = ?
ORDER BY stage, slot;
