-- +goose Up
-- Each owner can adopt one finished batch candidate at most once; the directory, current
-- synthetic snapshot and this mapping are committed by one voice-owned transaction.
CREATE TABLE writing_voice_candidate_adoptions (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 job_id TEXT NOT NULL REFERENCES generation_jobs(id),
 candidate_id TEXT NOT NULL,
 voice_id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(user_id,job_id,candidate_id),
 FOREIGN KEY(voice_id,user_id) REFERENCES voices(id,user_id)
);
-- +goose Down
DROP TABLE writing_voice_candidate_adoptions;
