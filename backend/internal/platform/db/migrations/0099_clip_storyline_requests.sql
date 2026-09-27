-- +goose Up
-- CLIP-133, CLIP-181: a storyline request's words join the record of what the owner asked the
-- AI for, as kind 'storyline'. SQLite widens a CHECK only by rebuilding the table; nothing
-- references it, so the rebuild moves every row as it is.
CREATE TABLE clip_project_requests_new (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('instruction', 'storyline', 'revision:flow', 'revision:narration', 'revision:both')),
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
-- A storyline request has nowhere to go in the old shape, so its rows are dropped.
CREATE TABLE clip_project_requests_old (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('instruction', 'revision:flow', 'revision:narration', 'revision:both')),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY(project_id, user_id) REFERENCES clip_projects(id, user_id) ON DELETE CASCADE
);
INSERT INTO clip_project_requests_old (id, project_id, user_id, kind, body, created_at)
SELECT id, project_id, user_id, kind, body, created_at FROM clip_project_requests WHERE kind != 'storyline';
DROP INDEX clip_project_requests_project;
DROP TABLE clip_project_requests;
ALTER TABLE clip_project_requests_old RENAME TO clip_project_requests;
CREATE INDEX clip_project_requests_project ON clip_project_requests(project_id, created_at);
