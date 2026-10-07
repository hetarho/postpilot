-- +goose Up
CREATE TABLE writing_tests (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 operation_key TEXT NOT NULL CHECK(operation_key <> ''),
 fingerprint TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('ab','knockout')),
 factor TEXT NOT NULL CHECK(factor IN ('model','voice','template','guideline')),
 model_stage TEXT NOT NULL DEFAULT '' CHECK(model_stage IN ('','observe','write')),
 count INTEGER NOT NULL CHECK(count IN (2,4,8,16)),
 status TEXT NOT NULL CHECK(status IN ('queued','running','partial','review','completed','cancelled','failed')),
 revision INTEGER NOT NULL DEFAULT 0 CHECK(revision >= 0),
 source_post_slug TEXT REFERENCES posts(slug) ON DELETE SET NULL,
 context TEXT NOT NULL CHECK(json_valid(context)),
 common_snapshot BLOB,
 common_hash TEXT NOT NULL,
 prompt_version TEXT NOT NULL,
 purge_fence INTEGER NOT NULL DEFAULT 0 CHECK(purge_fence >= 0),
 job_id TEXT NOT NULL DEFAULT '',
 winner_candidate_id TEXT,
 confirmed_credits INTEGER NOT NULL DEFAULT 0 CHECK(confirmed_credits >= 0),
 reserved_credits INTEGER NOT NULL DEFAULT 0 CHECK(reserved_credits >= 0),
 failure_reason TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 content_expires_at TEXT,
 UNIQUE(user_id,id),
 UNIQUE(user_id,operation_key),
 CHECK((factor='model' AND model_stage IN ('observe','write')) OR (factor<>'model' AND model_stage='')),
 CHECK((count=2 AND kind='ab') OR (count IN (4,8,16) AND kind='knockout')),
 FOREIGN KEY(user_id,source_post_slug) REFERENCES posts(user_id,slug),
 FOREIGN KEY(user_id,id,winner_candidate_id) REFERENCES writing_test_candidates(user_id,test_id,id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX writing_tests_history ON writing_tests(user_id,created_at DESC,id DESC);
CREATE TABLE writing_test_candidates (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 test_id TEXT NOT NULL,
 seed_position INTEGER NOT NULL CHECK(seed_position >= 0 AND seed_position < 16),
 source_kind TEXT NOT NULL CHECK(source_kind IN ('model','setting','authoring_candidate')),
 source_id TEXT NOT NULL,
 source_revision TEXT NOT NULL,
 semantic_key TEXT NOT NULL,
 frozen_variant BLOB,
 output BLOB,
 status TEXT NOT NULL CHECK(status IN ('pending','running','succeeded','failed','cancelled')),
 failure_reason TEXT NOT NULL DEFAULT '',
 accounting BLOB,
 started_at TEXT,
 finished_at TEXT,
 UNIQUE(user_id,test_id,id),
 UNIQUE(test_id,seed_position),
 UNIQUE(test_id,semantic_key),
 FOREIGN KEY(user_id,test_id) REFERENCES writing_tests(user_id,id) ON DELETE CASCADE
);
CREATE TABLE writing_test_matches (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 test_id TEXT NOT NULL,
 round INTEGER NOT NULL CHECK(round > 0 AND round <= 4),
 match_index INTEGER NOT NULL CHECK(match_index >= 0 AND match_index < 8),
 left_candidate_id TEXT NOT NULL,
 right_candidate_id TEXT NOT NULL,
 winner_candidate_id TEXT,
 decision_key TEXT,
 decided_at TEXT,
 UNIQUE(user_id,test_id,id),
 UNIQUE(test_id,round,match_index),
 UNIQUE(user_id,decision_key),
 CHECK(left_candidate_id<>right_candidate_id),
 CHECK(winner_candidate_id IS NULL OR winner_candidate_id IN (left_candidate_id,right_candidate_id)),
 CHECK((winner_candidate_id IS NULL AND decision_key IS NULL AND decided_at IS NULL)
    OR (winner_candidate_id IS NOT NULL AND decision_key IS NOT NULL AND decision_key<>'' AND decided_at IS NOT NULL)),
 FOREIGN KEY(user_id,test_id) REFERENCES writing_tests(user_id,id) ON DELETE CASCADE,
 FOREIGN KEY(user_id,test_id,left_candidate_id) REFERENCES writing_test_candidates(user_id,test_id,id),
 FOREIGN KEY(user_id,test_id,right_candidate_id) REFERENCES writing_test_candidates(user_id,test_id,id)
);
CREATE TABLE writing_test_publications (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 test_id TEXT NOT NULL,
 winner_candidate_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('save_setting','use_setting','adopt_model','apply_output')),
 request_key TEXT NOT NULL CHECK(request_key <> ''),
 fingerprint TEXT NOT NULL,
 target_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('pending','confirmed','conflict')),
 failure_reason TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 confirmed_at TEXT,
 UNIQUE(user_id,request_key),
 UNIQUE(user_id,test_id,winner_candidate_id,action),
 FOREIGN KEY(user_id,test_id) REFERENCES writing_tests(user_id,id) ON DELETE CASCADE,
 FOREIGN KEY(user_id,test_id,winner_candidate_id) REFERENCES writing_test_candidates(user_id,test_id,id)
);
-- Retention detaches a source only after private test payloads have been purged.
-- +goose StatementBegin
CREATE TRIGGER writing_tests_source_delete BEFORE DELETE ON posts
WHEN EXISTS(SELECT 1 FROM writing_tests WHERE user_id=OLD.user_id AND source_post_slug=OLD.slug AND purge_fence=0)
BEGIN SELECT RAISE(ABORT,'writing test content must be purged before source deletion'); END;
-- +goose StatementEnd
-- Every target owns its own receipt; a later manual change cannot invalidate a replay.
CREATE TABLE post_test_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 test_id TEXT NOT NULL,
 winner_candidate_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action='apply_output'),
 request_key TEXT NOT NULL CHECK(request_key<>''),
 fingerprint TEXT NOT NULL,
 post_slug TEXT NOT NULL,
 resulting_content_revision INTEGER NOT NULL CHECK(resulting_content_revision >= 0),
 receipt TEXT NOT NULL CHECK(json_valid(receipt)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,test_id,winner_candidate_id,action),
 UNIQUE(user_id,request_key)
);
CREATE TABLE provider_test_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 test_id TEXT NOT NULL,
 winner_candidate_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action='adopt_model'),
 request_key TEXT NOT NULL CHECK(request_key<>''),
 fingerprint TEXT NOT NULL,
 stage TEXT NOT NULL CHECK(stage IN ('observe','write')),
 model_ref TEXT NOT NULL,
 receipt TEXT NOT NULL CHECK(json_valid(receipt)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,test_id,winner_candidate_id,action),
 UNIQUE(user_id,request_key)
);
CREATE TABLE template_test_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 test_id TEXT NOT NULL,
 winner_candidate_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('save_setting','use_setting')),
 request_key TEXT NOT NULL CHECK(request_key<>''),
 fingerprint TEXT NOT NULL,
 target_id TEXT NOT NULL,
 receipt TEXT NOT NULL CHECK(json_valid(receipt)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,test_id,winner_candidate_id,action),
 UNIQUE(user_id,request_key)
);
CREATE TABLE guideline_test_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 test_id TEXT NOT NULL,
 winner_candidate_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('save_setting','use_setting')),
 request_key TEXT NOT NULL CHECK(request_key<>''),
 fingerprint TEXT NOT NULL,
 target_id TEXT NOT NULL,
 receipt TEXT NOT NULL CHECK(json_valid(receipt)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,test_id,winner_candidate_id,action),
 UNIQUE(user_id,request_key)
);
CREATE TABLE voice_test_publications (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 test_id TEXT NOT NULL,
 winner_candidate_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('save_setting','use_setting')),
 request_key TEXT NOT NULL CHECK(request_key<>''),
 fingerprint TEXT NOT NULL,
 target_id TEXT NOT NULL,
 receipt TEXT NOT NULL CHECK(json_valid(receipt)),
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,test_id,winner_candidate_id,action),
 UNIQUE(user_id,request_key)
);
-- +goose Down
-- Forward-only: older binaries must not destroy paid test history or receipts.
SELECT 1;
