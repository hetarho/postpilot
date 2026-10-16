-- +goose Up
ALTER TABLE images ADD COLUMN captured_at TEXT;
ALTER TABLE videos ADD COLUMN captured_at TEXT;
ALTER TABLE clip_source_leases ADD COLUMN captured_at TEXT;

-- +goose Down
ALTER TABLE clip_source_leases DROP COLUMN captured_at;
ALTER TABLE videos DROP COLUMN captured_at;
ALTER TABLE images DROP COLUMN captured_at;
