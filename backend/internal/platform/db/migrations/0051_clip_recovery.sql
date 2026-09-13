-- +goose Up
CREATE TABLE clip_recovery_states (
 project_id TEXT PRIMARY KEY REFERENCES clip_projects(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 job_id TEXT NOT NULL REFERENCES generation_jobs(id) ON DELETE CASCADE,
 state_json TEXT NOT NULL CHECK(json_valid(state_json) AND length(CAST(state_json AS BLOB))<=2097152)
);
-- Preserve legacy evidence before the next job's inspection trigger removes it.
-- Wrapping a valid near-limit checkpoint adds bytes. Keep its original inspection
-- intact, but do not let an oversized optional recovery copy prevent API startup.
WITH legacy AS (
SELECT c.project_id,c.user_id,c.job_id,json_object('Version',1,'JobID',c.job_id,'Legacy',json(c.checkpoint_json)) AS state_json
FROM clip_attempt_checkpoints c JOIN clip_projects p ON p.id=c.project_id
WHERE p.deleting=0 AND p.finalized_at IS NULL
)
INSERT INTO clip_recovery_states(project_id,user_id,job_id,state_json)
SELECT project_id,user_id,job_id,state_json FROM legacy
WHERE length(CAST(state_json AS BLOB))<=2097152;
-- +goose StatementBegin
CREATE TRIGGER clip_recovery_finalized AFTER UPDATE OF finalized_at,deleting ON clip_projects
WHEN NEW.finalized_at IS NOT NULL OR NEW.deleting<>0
BEGIN DELETE FROM clip_recovery_states WHERE project_id=NEW.id; END;
-- +goose StatementEnd
-- +goose Down
DROP TRIGGER clip_recovery_finalized;
DROP TABLE clip_recovery_states;
