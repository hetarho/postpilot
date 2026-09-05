-- +goose Up
-- A durable, non-sensitive diagnostic flag. The detailed provider text remains in the job
-- failure; the ledger only needs to count whether reasoning exhausted the completion budget.
ALTER TABLE usage_events ADD COLUMN reasoning_truncated INTEGER NOT NULL DEFAULT 0
  CHECK (reasoning_truncated IN (0, 1));

-- +goose Down
ALTER TABLE usage_events DROP COLUMN reasoning_truncated;
