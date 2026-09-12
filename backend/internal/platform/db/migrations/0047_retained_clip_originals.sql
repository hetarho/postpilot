-- +goose Up
ALTER TABLE clip_projects ADD COLUMN source_retention_expires_at TEXT;
ALTER TABLE clip_projects ADD COLUMN source_access_revoked_at TEXT;
ALTER TABLE clip_projects ADD COLUMN source_batch_id TEXT;
ALTER TABLE clip_source_batches ADD COLUMN put_expires_at TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_source_leases ADD COLUMN canonical_id TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_source_leases ADD COLUMN retention_expires_at TEXT;
ALTER TABLE clip_source_leases ADD COLUMN cleanup_pending INTEGER NOT NULL DEFAULT 0 CHECK(cleanup_pending IN (0,1));
UPDATE clip_source_batches SET put_expires_at=expires_at;
-- The old schema did not retain confirmation times. Give still-live confirmed
-- originals one full retention window at migration; never revive expired/unprotected objects.
UPDATE clip_source_leases SET canonical_id=id,retention_expires_at=(SELECT CASE WHEN state='cleanup_pending' OR julianday(expires_at)<=julianday('now') THEN expires_at ELSE strftime('%Y-%m-%dT%H:%M:%f000000Z','now','+24 hours') END FROM clip_source_batches WHERE id=batch_id) WHERE state='ready';
UPDATE clip_source_leases SET canonical_id=id WHERE canonical_id='';
UPDATE clip_projects SET source_batch_id=(SELECT id FROM clip_source_batches WHERE project_id=clip_projects.id AND state!='cleanup_pending');
UPDATE clip_projects SET source_access_revoked_at=updated_at WHERE deleting=1;
UPDATE clip_projects SET source_retention_expires_at=(SELECT MAX(l.retention_expires_at) FROM clip_source_leases l JOIN clip_source_batches b ON b.id=l.batch_id WHERE b.project_id=clip_projects.id AND b.user_id=clip_projects.user_id AND b.state!='cleanup_pending') WHERE source_access_revoked_at IS NULL;
DROP INDEX clip_source_active_project;
CREATE INDEX clip_source_project ON clip_source_batches(user_id,project_id,created_at);
CREATE UNIQUE INDEX clip_source_canonical ON clip_source_leases(batch_id,canonical_id);
-- Immutable attempt bindings deliberately survive project and job metadata deletion.
CREATE TABLE clip_source_attempts (
    job_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    batch_id TEXT NOT NULL,
    manifest_json TEXT NOT NULL,
    bound_at TEXT NOT NULL,
    released_at TEXT,
    FOREIGN KEY(batch_id,user_id) REFERENCES clip_source_batches(id,user_id) ON DELETE CASCADE
);
CREATE INDEX clip_source_attempt_active ON clip_source_attempts(batch_id) WHERE released_at IS NULL;
INSERT INTO clip_source_attempts(job_id,user_id,project_id,batch_id,manifest_json,bound_at)
SELECT job_id,user_id,project_id,id,'',created_at FROM clip_source_batches WHERE job_id IS NOT NULL;
-- A consumed quote belongs to an attempt; the next retry gets its own approved quote.
CREATE TABLE clip_generation_quotes_retained (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
    batch_id TEXT NOT NULL,
    input_digest TEXT NOT NULL,
    pricing_json TEXT NOT NULL,
    max_credits INTEGER NOT NULL CHECK(max_credits>=0),
    expires_at TEXT NOT NULL,
    consumed_job_id TEXT UNIQUE,
    FOREIGN KEY(batch_id,user_id) REFERENCES clip_source_batches(id,user_id) ON DELETE CASCADE
);
INSERT INTO clip_generation_quotes_retained SELECT * FROM clip_generation_quotes;
DROP TRIGGER clip_quote_owner;
DROP TRIGGER clip_quote_job_owner;
DROP TABLE clip_generation_quotes;
ALTER TABLE clip_generation_quotes_retained RENAME TO clip_generation_quotes;
CREATE UNIQUE INDEX clip_quote_unconsumed ON clip_generation_quotes(batch_id) WHERE consumed_job_id IS NULL;
-- +goose StatementBegin
CREATE TRIGGER clip_quote_owner BEFORE INSERT ON clip_generation_quotes
WHEN NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND user_id=NEW.user_id AND deleting=0 AND source_access_revoked_at IS NULL)
  OR NOT EXISTS (SELECT 1 FROM clip_source_batches WHERE id=NEW.batch_id AND user_id=NEW.user_id AND project_id=NEW.project_id)
BEGIN SELECT RAISE(ABORT,'clip quote target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_generation_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
      AND clip_project_id=NEW.project_id AND kind='generate_clip' AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip quote job unavailable'); END;
-- +goose StatementEnd

-- +goose Down
-- Preserve access denial when reverting to the earlier deletion-only protocol.
UPDATE clip_projects SET deleting=1 WHERE source_access_revoked_at IS NOT NULL;
DROP TRIGGER clip_quote_owner;
DROP TRIGGER clip_quote_job_owner;
ALTER TABLE clip_generation_quotes RENAME TO clip_generation_quotes_retained_down;
CREATE TABLE clip_generation_quotes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
    batch_id TEXT NOT NULL UNIQUE,
    input_digest TEXT NOT NULL,
    pricing_json TEXT NOT NULL,
    max_credits INTEGER NOT NULL CHECK (max_credits >= 0),
    expires_at TEXT NOT NULL,
    consumed_job_id TEXT UNIQUE,
    FOREIGN KEY(batch_id,user_id) REFERENCES clip_source_batches(id,user_id) ON DELETE CASCADE
);

INSERT INTO clip_generation_quotes SELECT q.* FROM clip_generation_quotes_retained_down q
WHERE q.id=(SELECT q2.id FROM clip_generation_quotes_retained_down q2 WHERE q2.batch_id=q.batch_id ORDER BY (q2.consumed_job_id IS NULL) DESC,q2.expires_at DESC,q2.id DESC LIMIT 1);
DROP TABLE clip_generation_quotes_retained_down;
-- +goose StatementBegin
CREATE TRIGGER clip_quote_owner BEFORE INSERT ON clip_generation_quotes
WHEN NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND user_id=NEW.user_id AND deleting=0)
  OR NOT EXISTS (SELECT 1 FROM clip_source_batches WHERE id=NEW.batch_id AND user_id=NEW.user_id AND project_id=NEW.project_id)
BEGIN SELECT RAISE(ABORT,'clip quote target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_generation_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
      AND clip_project_id=NEW.project_id AND kind='generate_clip' AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip quote job unavailable'); END;
-- +goose StatementEnd

DROP TABLE clip_source_attempts;
-- Older binaries support one current manifest per project; surplus identities remain queued for cleanup.
UPDATE clip_source_batches SET state='cleanup_pending' WHERE id!=(SELECT b.id FROM clip_source_batches b WHERE b.user_id=clip_source_batches.user_id AND b.project_id=clip_source_batches.project_id ORDER BY (b.state='consuming') DESC,b.created_at DESC,b.id DESC LIMIT 1);
DROP INDEX clip_source_project;
DROP INDEX clip_source_canonical;
CREATE UNIQUE INDEX clip_source_active_project ON clip_source_batches(user_id,project_id) WHERE state!='cleanup_pending';
ALTER TABLE clip_source_leases DROP COLUMN cleanup_pending;
ALTER TABLE clip_source_leases DROP COLUMN retention_expires_at;
ALTER TABLE clip_source_leases DROP COLUMN canonical_id;
ALTER TABLE clip_source_batches DROP COLUMN put_expires_at;
ALTER TABLE clip_projects DROP COLUMN source_batch_id;
ALTER TABLE clip_projects DROP COLUMN source_access_revoked_at;
ALTER TABLE clip_projects DROP COLUMN source_retention_expires_at;
