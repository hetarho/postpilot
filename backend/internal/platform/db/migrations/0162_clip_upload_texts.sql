-- +goose Up
CREATE TABLE clip_upload_texts (
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
CREATE TABLE clip_upload_text_quotes (
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
CREATE UNIQUE INDEX clip_upload_quote_unconsumed ON clip_upload_text_quotes(project_id,channel) WHERE consumed_job_id IS NULL;
-- +goose StatementBegin
CREATE TRIGGER clip_upload_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_upload_text_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
 SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
 AND clip_project_id=NEW.project_id AND kind='clip_upload_text' AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip upload text job unavailable'); END;
-- +goose StatementEnd
DROP TRIGGER clip_finalized_job_activate;
DROP TRIGGER clip_finalized_job_insert;
DROP TRIGGER clip_checkpoint_next_attempt;
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_activate BEFORE UPDATE OF dispatch_ready,status ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.kind <> 'clip_upload_text' AND NEW.status IN ('queued','running') AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_insert BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.kind <> 'clip_upload_text' AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_checkpoint_next_attempt AFTER INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.kind <> 'clip_upload_text'
BEGIN DELETE FROM clip_attempt_checkpoints WHERE project_id=NEW.clip_project_id; END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_upload_quote_job_owner;
DROP TABLE clip_upload_text_quotes;
DROP TABLE clip_upload_texts;
DROP TRIGGER clip_finalized_job_activate;
DROP TRIGGER clip_finalized_job_insert;
DROP TRIGGER clip_checkpoint_next_attempt;
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_activate BEFORE UPDATE OF dispatch_ready,status ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.status IN ('queued','running') AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_insert BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_checkpoint_next_attempt AFTER INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL
BEGIN DELETE FROM clip_attempt_checkpoints WHERE project_id=NEW.clip_project_id; END;
-- +goose StatementEnd
