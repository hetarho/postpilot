-- Runtime migration149 uses the transactional Go guard in migration_0149.go.
-- This canonical SQL also exposes the column to sqlc's schema parser.
-- +goose Up
ALTER TABLE generation_jobs ADD COLUMN wait_expires_at TEXT;

-- The additive nullable column is retained for rollback compatibility.
-- +goose Down
SELECT 1;
