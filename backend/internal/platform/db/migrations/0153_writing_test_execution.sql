-- +goose Up
CREATE TABLE writing_test_quotes (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 test_id TEXT,
 source_post_slug TEXT,
 fingerprint TEXT NOT NULL,
 request TEXT NOT NULL CHECK(json_valid(request)),
 plan BLOB,
 estimated_credits INTEGER NOT NULL CHECK(estimated_credits >= 0),
 free INTEGER NOT NULL CHECK(free IN (0,1)),
 consumed_request_key TEXT NOT NULL DEFAULT '',
 consumed_test_id TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 UNIQUE(user_id,id),
 FOREIGN KEY(user_id,test_id) REFERENCES writing_tests(user_id,id) ON DELETE CASCADE
);
CREATE INDEX writing_test_quotes_expiry ON writing_test_quotes(expires_at);
CREATE TABLE writing_test_attempts (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 test_id TEXT NOT NULL,
 request_key TEXT NOT NULL CHECK(request_key<>''),
 fingerprint TEXT NOT NULL,
 epoch INTEGER NOT NULL CHECK(epoch>=0),
 purge_fence INTEGER NOT NULL CHECK(purge_fence>=0),
 quote_id TEXT NOT NULL,
 job_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('prepared','queued','running','done','failed','cancelled','uncertain')),
 candidate_ids TEXT NOT NULL CHECK(json_valid(candidate_ids) AND json_type(candidate_ids)='array'),
 confirmed_credits INTEGER NOT NULL DEFAULT 0 CHECK(confirmed_credits>=0),
 settled INTEGER NOT NULL DEFAULT 0 CHECK(settled IN (0,1)),
 non_metered INTEGER NOT NULL DEFAULT 0 CHECK(non_metered IN (0,1)),
 failure_reason TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 UNIQUE(user_id,id),
 UNIQUE(user_id,test_id,id),
 UNIQUE(user_id,request_key),
 FOREIGN KEY(user_id,test_id) REFERENCES writing_tests(user_id,id) ON DELETE CASCADE,
 FOREIGN KEY(user_id,quote_id) REFERENCES writing_test_quotes(user_id,id)
);
-- A prepared attempt has no job yet, so empty job ids are not an identity.
CREATE UNIQUE INDEX writing_test_attempt_job ON writing_test_attempts(user_id,job_id) WHERE job_id<>'';
CREATE TABLE writing_test_attempt_checkpoints (
 user_id TEXT NOT NULL,
 test_id TEXT NOT NULL,
 attempt_id TEXT NOT NULL,
 slot_id TEXT NOT NULL,
 checkpoint BLOB,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(user_id,test_id,attempt_id,slot_id),
 FOREIGN KEY(user_id,test_id,attempt_id) REFERENCES writing_test_attempts(user_id,test_id,id) ON DELETE CASCADE
);
CREATE TABLE writing_test_operations (
 user_id TEXT NOT NULL,
 operation_key TEXT NOT NULL CHECK(operation_key<>''),
 test_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action='cancel'),
 fingerprint TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,operation_key),
 FOREIGN KEY(user_id,test_id) REFERENCES writing_tests(user_id,id) ON DELETE CASCADE
);
-- +goose Down
-- Forward-only: retain paid history, usage identity and publication recovery.
