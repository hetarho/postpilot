-- +goose Up
CREATE TABLE server_export_reservations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    plan_revision INTEGER NOT NULL CHECK (plan_revision > 0),
    job_id TEXT UNIQUE,
    coverage_id TEXT NOT NULL,
    window_start TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('reserved','committed','released')),
    created_at TEXT NOT NULL,
    FOREIGN KEY(user_id,coverage_id,window_start)
        REFERENCES server_export_windows(user_id,coverage_id,window_start) ON DELETE CASCADE
);
CREATE INDEX idx_server_export_reservations_open
ON server_export_reservations(state,job_id) WHERE state='reserved';

-- +goose Down
DROP TABLE server_export_reservations;
