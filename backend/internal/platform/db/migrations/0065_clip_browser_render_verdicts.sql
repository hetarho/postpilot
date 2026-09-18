-- +goose Up
CREATE TABLE clip_browser_renders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
    plan_revision INTEGER NOT NULL CHECK (plan_revision > 0),
    ratio TEXT NOT NULL,
    duration_ms INTEGER NOT NULL,
    has_audio INTEGER NOT NULL CHECK (has_audio IN (0,1)),
    created_at TEXT NOT NULL,
    verdict_json TEXT,
    reported_at TEXT
);
CREATE INDEX clip_browser_renders_project ON clip_browser_renders(project_id);

-- +goose Down
DROP TABLE clip_browser_renders;
