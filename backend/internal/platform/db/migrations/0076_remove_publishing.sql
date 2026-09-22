-- +goose Up
-- The T312 bridge must have removed every server-owned publishing row before this
-- final build can start. Keep the assertion and all drops in one migration transaction
-- so a dirty installation retains every record and gets one actionable error.
CREATE TABLE publishing_final_removal_guard (row_count INTEGER NOT NULL);

-- +goose StatementBegin
CREATE TRIGGER publishing_cleanup_required
BEFORE INSERT ON publishing_final_removal_guard
WHEN NEW.row_count <> 0
BEGIN
  SELECT RAISE(ABORT, 'publishing cleanup required before final removal');
END;
-- +goose StatementEnd

INSERT INTO publishing_final_removal_guard(row_count)
SELECT
  (SELECT COUNT(*) FROM publish_assets) +
  (SELECT COUNT(*) FROM publish_jobs) +
  (SELECT COUNT(*) FROM publish_job_ids) +
  (SELECT COUNT(*) FROM publishing_agents) +
  (SELECT COUNT(*) FROM publishing_pairings);

DROP TRIGGER publishing_cleanup_required;
DROP TABLE publishing_final_removal_guard;

DROP TRIGGER IF EXISTS publishing_retired_asset_insert;
DROP TRIGGER IF EXISTS publishing_retired_job_reactivate;
DROP TRIGGER IF EXISTS publishing_retired_job_insert;
DROP TRIGGER IF EXISTS publishing_retired_job_id_insert;
DROP TRIGGER IF EXISTS publishing_retired_agent_rearm;
DROP TRIGGER IF EXISTS publishing_retired_agent_rearm_legacy;
DROP TRIGGER IF EXISTS publishing_retired_agent_insert;
DROP TRIGGER IF EXISTS publishing_agents_retired_executor_sync;
DROP TRIGGER IF EXISTS publishing_retired_pairing_reopen;
DROP TRIGGER IF EXISTS publishing_retired_pairing_insert;

DROP TABLE publish_assets;
DROP TABLE publish_jobs;
DROP TABLE publish_job_ids;
DROP TABLE publishing_agents;
DROP TABLE publishing_pairings;

-- +goose Down
-- Retirement is irreversible. Lowering the recorded version must not recreate tables,
-- credentials, routes or executable publishing capability.
SELECT 1;
