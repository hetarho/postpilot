-- +goose Up
-- Add the executor-neutral capability without renaming the legacy column. A
-- rollback image can therefore continue reading the old schema after this
-- migration has run; a later cleanup migration may remove it after the rollback
-- window closes. Protobuf keeps field number 8 for wire compatibility.
ALTER TABLE publishing_agents ADD COLUMN executor_version TEXT NOT NULL DEFAULT '';

-- A rolled-back server still writes the retired physical version column. Keep
-- such a sync fail-closed even if a deterministic version had previously been
-- recorded, so rolling forward again cannot accidentally trust the old daemon.
-- +goose StatementBegin
CREATE TRIGGER publishing_agents_retired_executor_sync
AFTER UPDATE OF hermes_version ON publishing_agents
BEGIN
    UPDATE publishing_agents
    SET compatibility_ready=0, executor_version=''
    WHERE id=NEW.id;
END;
-- +goose StatementEnd

-- End every old in-flight lease at the deployment boundary. A pre-commit job
-- keeps its frozen manifest for explicit retry after the replacement is ready;
-- a job that crossed the fence becomes outcome_unknown and can never auto-retry.
UPDATE publish_jobs
SET status=CASE
        WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published') THEN 'outcome_unknown'
        ELSE 'needs_attention'
    END,
    progress_seq=progress_seq+1,
    manifest_json=CASE
        WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published') THEN NULL
        ELSE manifest_json
    END,
    lease_token_hash=NULL,
    lease_expires_at=NULL,
    error_code='executor_retired',
    error_message='The retired local publishing executor was disabled during migration.',
    updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE status='running';

-- The retired executor's capability result is not evidence for the replacement.
-- Existing connections must pass the new local probe before they can publish.
UPDATE publishing_agents SET compatibility_ready = 0, executor_version = '';

-- +goose Down
DROP TRIGGER publishing_agents_retired_executor_sync;
-- Schema rollback deliberately leaves every connection unready. Restoring an
-- old capability result or replaying an old in-flight lease would be unsafe;
-- production rollbacks keep migration 0015 applied and never re-enable the
-- retired executor.
UPDATE publishing_agents SET compatibility_ready=0;
ALTER TABLE publishing_agents DROP COLUMN executor_version;
