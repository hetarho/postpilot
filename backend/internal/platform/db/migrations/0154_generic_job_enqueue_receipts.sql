-- +goose Up
CREATE TABLE job_enqueue_receipts (
 job_id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 fingerprint TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('claimed','committed','abandoned')),
 created_at TEXT NOT NULL
);
CREATE INDEX job_enqueue_receipts_owner ON job_enqueue_receipts(user_id,job_id);

-- +goose Down
DROP TABLE job_enqueue_receipts;
