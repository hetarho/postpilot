-- Queries for the provider context's recommendation sets (MODEL-69). ASCII only: sqlc
-- rewrites named parameters by byte offset, so a multi-byte comment breaks the whole file.

-- name: ListRecommendationSets :many
SELECT id, label, position
FROM recommendation_sets
ORDER BY position, created_at, id;

-- name: ListRecommendationSetSlots :many
SELECT set_id, stage, slot, provider_id, model_id
FROM recommendation_set_slots
ORDER BY set_id, stage, slot;

-- name: CountRecommendationSets :one
SELECT count(*) FROM recommendation_sets;

-- name: NextRecommendationSetPosition :one
SELECT CAST(COALESCE(MAX(position), 0) + 1 AS INTEGER) FROM recommendation_sets;

-- name: InsertRecommendationSet :exec
INSERT INTO recommendation_sets (id, label, position, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateRecommendationSetLabel :execrows
UPDATE recommendation_sets SET label = ?, updated_at = ? WHERE id = ?;

-- name: SetRecommendationSetPosition :exec
UPDATE recommendation_sets SET position = ? WHERE id = ?;

-- name: DeleteRecommendationSet :execrows
DELETE FROM recommendation_sets WHERE id = ?;

-- name: DeleteRecommendationSetSlots :exec
DELETE FROM recommendation_set_slots WHERE set_id = ?;

-- name: InsertRecommendationSetSlot :exec
INSERT INTO recommendation_set_slots (set_id, stage, slot, provider_id, model_id)
VALUES (?, ?, ?, ?, ?);
