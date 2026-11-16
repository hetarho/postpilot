-- +goose Up
-- A clip project has no channel (CLIP-209). Each project keeps the upload text
-- stored for the channel it was on, body and tags as written; a text or an
-- unconsumed quote stored for its other channel was never reachable and is
-- removed. Consumed quotes stay as the record of their jobs. Posted URLs are
-- untouched.
DELETE FROM clip_upload_texts WHERE channel <> (SELECT p.channel FROM clip_projects p WHERE p.id = clip_upload_texts.project_id);
DELETE FROM clip_upload_text_quotes WHERE consumed_job_id IS NULL AND channel <> (SELECT p.channel FROM clip_projects p WHERE p.id = clip_upload_text_quotes.project_id);

CREATE TABLE clip_upload_texts_next (
 project_id TEXT NOT NULL PRIMARY KEY,
 user_id TEXT NOT NULL,
 body TEXT NOT NULL,
 tags TEXT NOT NULL DEFAULT '[]',
 edited INTEGER NOT NULL DEFAULT 0 CHECK(edited IN (0,1)),
 updated_at TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1 CHECK(revision > 0),
 last_job_id TEXT NOT NULL DEFAULT '',
 FOREIGN KEY(project_id,user_id) REFERENCES clip_projects(id,user_id) ON DELETE CASCADE
);
INSERT INTO clip_upload_texts_next(project_id,user_id,body,tags,edited,updated_at,revision,last_job_id)
 SELECT project_id,user_id,body,tags,edited,updated_at,revision,last_job_id FROM clip_upload_texts;
DROP TABLE clip_upload_texts;
ALTER TABLE clip_upload_texts_next RENAME TO clip_upload_texts;

DROP TRIGGER clip_upload_quote_job_owner;
DROP INDEX clip_upload_quote_unconsumed;
CREATE TABLE clip_upload_text_quotes_next (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 project_id TEXT NOT NULL,
 expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
 input_digest TEXT NOT NULL,
 pricing_json TEXT NOT NULL,
 max_credits INTEGER NOT NULL CHECK(max_credits >= 0),
 expires_at TEXT NOT NULL,
 consumed_job_id TEXT UNIQUE,
 FOREIGN KEY(project_id,user_id) REFERENCES clip_projects(id,user_id) ON DELETE CASCADE
);
INSERT INTO clip_upload_text_quotes_next(id,user_id,project_id,expected_revision,input_digest,pricing_json,max_credits,expires_at,consumed_job_id)
 SELECT id,user_id,project_id,expected_revision,input_digest,pricing_json,max_credits,expires_at,consumed_job_id FROM clip_upload_text_quotes;
DROP TABLE clip_upload_text_quotes;
ALTER TABLE clip_upload_text_quotes_next RENAME TO clip_upload_text_quotes;
CREATE UNIQUE INDEX clip_upload_quote_unconsumed ON clip_upload_text_quotes(project_id) WHERE consumed_job_id IS NULL;
-- +goose StatementBegin
CREATE TRIGGER clip_upload_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_upload_text_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
 SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
 AND clip_project_id=NEW.project_id AND kind='clip_upload_text' AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip upload text job unavailable'); END;
-- +goose StatementEnd

ALTER TABLE clip_projects DROP COLUMN channel;

-- +goose Down
-- The columns come back defaulting to 'naver_clip'; folded rows are not restored.
ALTER TABLE clip_projects ADD COLUMN channel TEXT NOT NULL DEFAULT 'naver_clip' CHECK(channel IN ('naver_clip','instagram_clip'));

DROP TRIGGER clip_upload_quote_job_owner;
DROP INDEX clip_upload_quote_unconsumed;
CREATE TABLE clip_upload_text_quotes_prev (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 project_id TEXT NOT NULL,
 channel TEXT NOT NULL CHECK(channel IN ('naver_clip','instagram_clip')),
 expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
 input_digest TEXT NOT NULL,
 pricing_json TEXT NOT NULL,
 max_credits INTEGER NOT NULL CHECK(max_credits >= 0),
 expires_at TEXT NOT NULL,
 consumed_job_id TEXT UNIQUE,
 FOREIGN KEY(project_id,user_id) REFERENCES clip_projects(id,user_id) ON DELETE CASCADE
);
INSERT INTO clip_upload_text_quotes_prev(id,user_id,project_id,channel,expected_revision,input_digest,pricing_json,max_credits,expires_at,consumed_job_id)
 SELECT id,user_id,project_id,'naver_clip',expected_revision,input_digest,pricing_json,max_credits,expires_at,consumed_job_id FROM clip_upload_text_quotes;
DROP TABLE clip_upload_text_quotes;
ALTER TABLE clip_upload_text_quotes_prev RENAME TO clip_upload_text_quotes;
CREATE UNIQUE INDEX clip_upload_quote_unconsumed ON clip_upload_text_quotes(project_id,channel) WHERE consumed_job_id IS NULL;
-- +goose StatementBegin
CREATE TRIGGER clip_upload_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_upload_text_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
 SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
 AND clip_project_id=NEW.project_id AND kind='clip_upload_text' AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip upload text job unavailable'); END;
-- +goose StatementEnd

CREATE TABLE clip_upload_texts_prev (
 project_id TEXT NOT NULL,
 user_id TEXT NOT NULL,
 channel TEXT NOT NULL CHECK(channel IN ('naver_clip','instagram_clip')),
 body TEXT NOT NULL,
 tags TEXT NOT NULL DEFAULT '[]',
 edited INTEGER NOT NULL DEFAULT 0 CHECK(edited IN (0,1)),
 updated_at TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1 CHECK(revision > 0),
 last_job_id TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(project_id,channel),
 FOREIGN KEY(project_id,user_id) REFERENCES clip_projects(id,user_id) ON DELETE CASCADE
);
INSERT INTO clip_upload_texts_prev(project_id,user_id,channel,body,tags,edited,updated_at,revision,last_job_id)
 SELECT project_id,user_id,'naver_clip',body,tags,edited,updated_at,revision,last_job_id FROM clip_upload_texts;
DROP TABLE clip_upload_texts;
ALTER TABLE clip_upload_texts_prev RENAME TO clip_upload_texts;
