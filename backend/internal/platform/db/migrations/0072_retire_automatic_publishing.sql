-- +goose Up
-- Automatic publishing is permanently retired. This migration is the rollback floor:
-- it closes every durable capability before the bridge keeps only read/report access.

UPDATE publishing_pairings
SET consumed_at = COALESCE(consumed_at, strftime('%Y-%m-%dT%H:%M:%fZ','now'));

UPDATE publishing_agents
SET compatibility_ready = 0,
    executor_version = '',
    revoked_at = COALESCE(revoked_at, strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now');

UPDATE publish_jobs
SET status = CASE
      WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published')
        THEN 'outcome_unknown'
      ELSE 'canceled'
    END,
    progress_seq = progress_seq + 1,
    manifest_json = NULL,
    lease_token_hash = NULL,
    lease_expires_at = NULL,
    error_code = NULL,
    error_message = NULL,
    error_reason = CASE
      WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published')
        THEN 'PUBLISH_OUTCOME_UNKNOWN'
      ELSE 'PUBLISH_AGENT_UNAVAILABLE'
    END,
    error_params = '{}',
    technical_detail = 'Automatic publishing was retired at the server cutover.',
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE status IN ('queued','running','needs_attention');

-- Old binaries cannot mint or restore a capability after this schema is present.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_pairing_insert
BEFORE INSERT ON publishing_pairings
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_pairing_reopen
BEFORE UPDATE ON publishing_pairings
WHEN NEW.consumed_at IS NULL
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_agent_insert
BEFORE INSERT ON publishing_agents
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS publishing_retired_agent_rearm_legacy;

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_agent_rearm
BEFORE UPDATE ON publishing_agents
WHEN NEW.revoked_at IS NULL
  OR NEW.compatibility_ready <> 0
  OR NEW.executor_version <> ''
  OR NEW.token_hash <> OLD.token_hash
  OR NEW.last_seen_at IS NOT OLD.last_seen_at
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_job_id_insert
BEFORE INSERT ON publish_job_ids
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_job_insert
BEFORE INSERT ON publish_jobs
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_asset_insert
BEFORE INSERT ON publish_assets
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

-- Reads, conservative terminal corrections and cleanup DELETEs remain available.
-- Active status, lease restoration and a new commit fence never do.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_job_reactivate
BEFORE UPDATE ON publish_jobs
WHEN NEW.status IN ('queued','running','needs_attention')
  OR NEW.lease_token_hash IS NOT NULL
  OR NEW.lease_expires_at IS NOT NULL
  OR (OLD.committed_at IS NULL AND NEW.committed_at IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd

-- +goose Down
-- Deliberately leave the revoked data and guards in place. Goose may lower its recorded
-- version for historical migration tests, but rolling back one application version does
-- not restore any executor capability. Reapplying Up is idempotent over these guards.
DROP TRIGGER IF EXISTS publishing_retired_agent_rearm;

-- Older schemas do not have executor_version. Keep the remaining revocation, token and
-- heartbeat fences so a binary rollback still cannot restore an agent capability.
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS publishing_retired_agent_rearm_legacy
BEFORE UPDATE ON publishing_agents
WHEN NEW.revoked_at IS NULL
  OR NEW.compatibility_ready <> 0
  OR NEW.token_hash <> OLD.token_hash
  OR NEW.last_seen_at IS NOT OLD.last_seen_at
BEGIN
  SELECT RAISE(ABORT, 'automatic publishing retired');
END;
-- +goose StatementEnd
