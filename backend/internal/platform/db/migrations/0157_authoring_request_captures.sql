-- +goose Up
-- Safe request evidence remains private to its existing authoring operation.
-- The irreversible per-operation purge bit fences in-flight and replayed callbacks.
ALTER TABLE configuration_authoring_operations ADD COLUMN request_capture TEXT CHECK(request_capture IS NULL OR json_valid(request_capture));
ALTER TABLE configuration_authoring_operations ADD COLUMN capture_revision INTEGER CHECK(capture_revision IS NULL OR capture_revision >= 0);
ALTER TABLE configuration_authoring_operations ADD COLUMN capture_purged INTEGER NOT NULL DEFAULT 0 CHECK(capture_purged IN (0,1));

-- +goose Down
ALTER TABLE configuration_authoring_operations DROP COLUMN capture_purged;
ALTER TABLE configuration_authoring_operations DROP COLUMN capture_revision;
ALTER TABLE configuration_authoring_operations DROP COLUMN request_capture;
