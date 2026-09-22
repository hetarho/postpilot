-- +goose Up
-- The all-scope leaderboard reads every account's verdicts inside a rolling window
-- (MODEL-38), so its scan is by stage and decision time rather than by owner. The existing
-- model_experiments_user_stage_created index serves the per-account boards and the history
-- list; neither can serve this one.
CREATE INDEX model_experiments_stage_decided
ON model_experiments(stage, decided_at)
WHERE decided_at IS NOT NULL;

-- +goose Down
DROP INDEX model_experiments_stage_decided;
