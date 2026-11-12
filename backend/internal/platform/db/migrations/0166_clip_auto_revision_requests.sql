-- +goose Up
-- CLIP-131, CLIP-133: a revision request that names no target is recorded as kind
-- 'revision:auto' beside the earlier targeted kinds, which keep their meaning. SQLite widens
-- a CHECK only by rebuilding the table; nothing references it, so the rebuild moves every
-- row as it is.
CREATE TABLE clip_project_requests_new (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('instruction', 'storyline', 'revision:auto', 'revision:flow', 'revision:narration', 'revision:both')),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY(project_id, user_id) REFERENCES clip_projects(id, user_id) ON DELETE CASCADE
);
INSERT INTO clip_project_requests_new (id, project_id, user_id, kind, body, created_at)
SELECT id, project_id, user_id, kind, body, created_at FROM clip_project_requests;
DROP INDEX clip_project_requests_project;
DROP TABLE clip_project_requests;
ALTER TABLE clip_project_requests_new RENAME TO clip_project_requests;
CREATE INDEX clip_project_requests_project ON clip_project_requests(project_id, created_at);

-- +goose Down
-- The old shape has no auto kind; those requests asked for both documents at most, so they
-- read back as 'revision:both'.
CREATE TABLE clip_project_requests_old (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('instruction', 'storyline', 'revision:flow', 'revision:narration', 'revision:both')),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY(project_id, user_id) REFERENCES clip_projects(id, user_id) ON DELETE CASCADE
);
INSERT INTO clip_project_requests_old (id, project_id, user_id, kind, body, created_at)
SELECT id, project_id, user_id, CASE kind WHEN 'revision:auto' THEN 'revision:both' ELSE kind END, body, created_at FROM clip_project_requests;
DROP INDEX clip_project_requests_project;
DROP TABLE clip_project_requests;
ALTER TABLE clip_project_requests_old RENAME TO clip_project_requests;
CREATE INDEX clip_project_requests_project ON clip_project_requests(project_id, created_at);
