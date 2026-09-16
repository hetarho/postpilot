-- +goose Up
-- What the owner asked the AI for, verbatim, kept with the project (CLIP-133).
-- One row per ACCEPTED request: the instruction a generation froze, or the words
-- of a revision. Nothing here is written by a save — an instruction typed and
-- never used leaves no row — and nothing is normalised, trimmed or deduplicated,
-- because the record exists to answer "what did I ask for that time".
--
-- `kind` is 'instruction' or 'revision:' plus the target the request named, so a
-- reader can tell the two apart and see which document a revision was about. An
-- empty body under 'instruction' is the record of a clip written WITHOUT one: the
-- absence is itself an answer, so it is stored rather than skipped.
--
-- It is the project's, not the sources': it outlives the 24-hour original
-- retention (CLIP-73) and goes only when the project does (CLIP-24). It is owner
-- content, never diagnostics — the checkpoints CLIP-88 bounds stay where they are.
CREATE TABLE clip_project_requests (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('instruction', 'revision:flow', 'revision:narration', 'revision:both')),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY(project_id, user_id) REFERENCES clip_projects(id, user_id) ON DELETE CASCADE
);
CREATE INDEX clip_project_requests_project ON clip_project_requests(project_id, created_at);

-- +goose Down
DROP INDEX clip_project_requests_project;
DROP TABLE clip_project_requests;
