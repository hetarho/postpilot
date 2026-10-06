-- +goose Up
-- Publication receipts survive target deletion and commit with each owned entity.
CREATE TABLE template_authoring_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>=0),
 publication_key TEXT NOT NULL,
 target_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,session_id,revision),
 UNIQUE(user_id,publication_key)
);
CREATE TABLE video_template_authoring_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>=0),
 publication_key TEXT NOT NULL,
 target_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,session_id,revision),
 UNIQUE(user_id,publication_key)
);
CREATE TABLE guideline_authoring_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>=0),
 publication_key TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('post','clip')),
 target_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,session_id,revision),
 UNIQUE(user_id,publication_key)
);
CREATE TABLE voice_authoring_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision>=0),
 publication_key TEXT NOT NULL,
 target_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,session_id,revision),
 UNIQUE(user_id,publication_key)
);
-- +goose Down
DROP TABLE voice_authoring_publications;
DROP TABLE guideline_authoring_publications;
DROP TABLE video_template_authoring_publications;
DROP TABLE template_authoring_publications;
