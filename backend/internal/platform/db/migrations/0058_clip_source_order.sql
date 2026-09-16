-- +goose Up
-- The owner arranges the footage in ① and the writer takes the sources in that
-- order (CLIP-136). Position is per batch, like the lease itself, so a
-- discarded batch takes its arrangement with it. 0 for every existing lease
-- means "never arranged": the confirmation order the `ordinal` holds stands.
ALTER TABLE clip_source_leases ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE clip_source_leases DROP COLUMN position;
