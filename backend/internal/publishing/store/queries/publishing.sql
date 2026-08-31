-- name: CreatePairingBelowLimit :execrows
INSERT INTO publishing_pairings(code_hash,user_id,label,expires_at,created_at)
SELECT sqlc.arg(code_hash),sqlc.arg(user_id),sqlc.arg(label),sqlc.arg(pairing_expires_at),sqlc.arg(created_at)
WHERE (SELECT count(*) FROM publishing_pairings AS pending
       WHERE pending.consumed_at IS NULL AND pending.expires_at > sqlc.arg(now))
      < CAST(sqlc.arg(max_pending) AS INTEGER);

-- name: ConsumePairing :one
UPDATE publishing_pairings SET consumed_at = ?
WHERE code_hash = ? AND consumed_at IS NULL AND expires_at > ?
RETURNING user_id,label;

-- name: CreateAgent :exec
INSERT INTO publishing_agents(
  id,user_id,token_hash,label,platform,browser_label,created_at,updated_at
) VALUES(?,?,?,?,'naver_blog',?,?,?);

-- name: GetActiveAgentByTokenHash :one
SELECT * FROM publishing_agents WHERE token_hash = ? AND revoked_at IS NULL;

-- name: GetOwnedAgent :one
SELECT * FROM publishing_agents WHERE id = ? AND user_id = ?;

-- name: ListAgentsByUser :many
SELECT * FROM publishing_agents WHERE user_id = ? ORDER BY created_at DESC,id DESC;

-- name: UpdateOwnedAgentDefaults :execrows
UPDATE publishing_agents
SET label=?,default_category_id=?,default_visibility=?,updated_at=?
WHERE id=? AND user_id=? AND revoked_at IS NULL;

-- name: SyncAgentProfile :execrows
UPDATE publishing_agents
SET platform_account_id=?,platform_account_label=?,browser_label=?,categories_json=?,
    default_category_id=?,default_visibility=?,compatibility_ready=?,hermes_version=?,
    last_seen_at=?,updated_at=?
WHERE id=? AND user_id=? AND revoked_at IS NULL;

-- name: TouchAgent :execrows
UPDATE publishing_agents SET last_seen_at=?,updated_at=?
WHERE id=? AND user_id=? AND revoked_at IS NULL;

-- name: RevokeOwnedAgent :execrows
UPDATE publishing_agents SET revoked_at=?,updated_at=?
WHERE id=? AND user_id=? AND revoked_at IS NULL;

-- name: LockReadyAgentForPublish :execrows
-- This no-op update reserves the serialized writer before Start re-reads the
-- post and agent through their owning contexts. No relevant mutation can slip
-- between that guard and CreatePublishJob in the same transaction.
UPDATE publishing_agents SET updated_at=updated_at
WHERE id=? AND user_id=? AND revoked_at IS NULL AND compatibility_ready=1;

-- name: ReservePublishJobID :execrows
INSERT INTO publish_job_ids(id,user_id,created_at) VALUES(?,?,?)
ON CONFLICT(id) DO NOTHING;

-- name: ReleaseUnusedPublishJobID :execrows
DELETE FROM publish_job_ids
WHERE publish_job_ids.id=? AND publish_job_ids.user_id=?
  AND NOT EXISTS (SELECT 1 FROM publish_jobs WHERE publish_jobs.id=publish_job_ids.id);

-- name: CreatePublishJob :exec
INSERT INTO publish_jobs(
  id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,progress_seq,attempt,
  content_revision,target_language,content_language,voice_source_language,
  manifest_json,settings_json,created_at,updated_at
) VALUES(?,?,?,?,?,'naver_blog','queued','queued',0,0,?,?,?,?,?,?,?,?);

-- name: CreatePublishAsset :exec
INSERT INTO publish_assets(job_id,user_id,ordinal,filename,source_filename,staged_key,bytes,created_at)
VALUES(?,?,?,?,?,?,?,?);

-- name: RetryAttentionPublishJob :execrows
UPDATE publish_jobs
SET status='queued',stage='queued',progress_seq=0,
    error_code=NULL,error_message=NULL,error_reason=NULL,error_params=NULL,technical_detail=NULL,
    updated_at=?
WHERE publish_jobs.id=? AND publish_jobs.user_id=? AND publish_jobs.status='needs_attention'
  AND publish_jobs.committed_at IS NULL AND publish_jobs.manifest_json IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM publishing_agents AS active_agent
    WHERE active_agent.id=publish_jobs.agent_id
      AND active_agent.user_id=publish_jobs.user_id
      AND active_agent.revoked_at IS NULL
      AND active_agent.compatibility_ready=1
      AND TRIM(active_agent.browser_label)<>''
      AND active_agent.platform_account_id=json_extract(publish_jobs.manifest_json,'$.expected_platform_account_id')
      AND EXISTS (
        SELECT 1 FROM json_each(active_agent.categories_json) AS frozen_category
        WHERE json_extract(frozen_category.value,'$.ID')=json_extract(publish_jobs.manifest_json,'$.category_id')
          AND json_extract(frozen_category.value,'$.Name')=json_extract(publish_jobs.manifest_json,'$.category_name')
      )
      AND EXISTS (
        SELECT 1 FROM json_each(active_agent.categories_json) AS default_category
        WHERE json_extract(default_category.value,'$.ID')=active_agent.default_category_id
          AND TRIM(COALESCE(json_extract(default_category.value,'$.Name'),''))<>''
      )
  );

-- name: GetOwnedPublishJob :one
SELECT * FROM publish_jobs WHERE id=? AND user_id=?;

-- name: GetLatestPublishJobForPost :one
SELECT * FROM publish_jobs WHERE post_slug=? AND user_id=? AND post_created_at=?
ORDER BY created_at DESC,id DESC LIMIT 1;

-- name: GetLatestPublishJobForDeletedPost :one
-- Used only after the post context confirms that no current slug incarnation
-- exists, so a retained frozen needs-attention job remains explicitly retryable.
SELECT * FROM publish_jobs WHERE post_slug=? AND user_id=?
ORDER BY created_at DESC,id DESC LIMIT 1;

-- name: ListPublishAssets :many
SELECT * FROM publish_assets WHERE job_id=? ORDER BY ordinal;

-- name: ClaimQueuedPublishJob :one
UPDATE publish_jobs
SET status='running',stage='claimed',progress_seq=1,attempt=attempt+1,
    lease_token_hash=?,lease_expires_at=?,claimed_at=COALESCE(claimed_at,?),updated_at=?
WHERE id=(
  SELECT j.id FROM publish_jobs j
  JOIN publishing_agents a ON a.id=j.agent_id AND a.user_id=j.user_id
  WHERE j.status='queued' AND a.id=? AND a.user_id=? AND a.revoked_at IS NULL
  ORDER BY j.created_at,j.id LIMIT 1
)
RETURNING *;

-- name: ListRetryablePublishJobs :many
SELECT * FROM publish_jobs
WHERE user_id=? AND status='needs_attention' AND committed_at IS NULL
  AND manifest_json IS NOT NULL
ORDER BY updated_at DESC,id DESC;

-- name: RenewPublishLease :execrows
UPDATE publish_jobs SET lease_expires_at=?,updated_at=?
WHERE publish_jobs.id=? AND publish_jobs.user_id=? AND publish_jobs.agent_id=? AND publish_jobs.status='running'
  AND publish_jobs.lease_token_hash=? AND publish_jobs.lease_expires_at>?
  AND EXISTS (
    SELECT 1 FROM publishing_agents AS active_agent
    WHERE active_agent.id=publish_jobs.agent_id
      AND active_agent.user_id=publish_jobs.user_id
      AND active_agent.revoked_at IS NULL
  );

-- name: UpdatePublishProgress :execrows
UPDATE publish_jobs
SET stage=sqlc.arg(next_stage),progress_seq=sqlc.arg(next_seq),
    committed_at=CASE WHEN sqlc.arg(next_stage)='committing' THEN COALESCE(committed_at,sqlc.arg(committed_at)) ELSE committed_at END,
    updated_at=sqlc.arg(updated_at)
WHERE publish_jobs.id=sqlc.arg(id) AND publish_jobs.user_id=sqlc.arg(user_id)
  AND publish_jobs.agent_id=sqlc.arg(agent_id) AND publish_jobs.status='running'
  AND publish_jobs.lease_token_hash=sqlc.arg(lease_token_hash)
  AND publish_jobs.lease_expires_at>sqlc.arg(now)
  AND publish_jobs.stage=sqlc.arg(current_stage)
  AND publish_jobs.progress_seq=sqlc.arg(current_seq)
  AND publish_jobs.progress_seq<sqlc.arg(next_seq)
  AND EXISTS (
    SELECT 1 FROM publishing_agents AS active_agent
    WHERE active_agent.id=publish_jobs.agent_id
      AND active_agent.user_id=publish_jobs.user_id
      AND active_agent.revoked_at IS NULL
  );

-- name: CompletePublishJob :execrows
UPDATE publish_jobs
SET status='published',stage='published',progress_seq=?,platform_post_url=?,published_at=?,
    manifest_json=NULL,lease_token_hash=NULL,lease_expires_at=NULL,
    error_code=NULL,error_message=NULL,error_reason=NULL,error_params=NULL,technical_detail=NULL,updated_at=?
WHERE publish_jobs.id=? AND publish_jobs.user_id=? AND publish_jobs.agent_id=?
  AND publish_jobs.status='running' AND publish_jobs.stage='verifying'
  AND publish_jobs.lease_token_hash=? AND publish_jobs.lease_expires_at>?
  AND publish_jobs.progress_seq<?
  AND EXISTS (
    SELECT 1 FROM publishing_agents AS active_agent
    WHERE active_agent.id=publish_jobs.agent_id
      AND active_agent.user_id=publish_jobs.user_id
      AND active_agent.revoked_at IS NULL
  );

-- name: FailPublishJob :execrows
UPDATE publish_jobs
SET status=CASE
      WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published') THEN sqlc.arg(commit_status)
      ELSE sqlc.arg(precommit_status)
    END,
    progress_seq=sqlc.arg(next_seq),
    error_code=NULL,
    error_message=NULL,
    error_reason=CASE
      WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published') THEN sqlc.arg(commit_error_reason)
      ELSE sqlc.arg(precommit_error_reason)
    END,
    error_params=CASE
      WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published') THEN sqlc.arg(commit_error_params)
      ELSE sqlc.arg(precommit_error_params)
    END,
    technical_detail=CASE
      WHEN committed_at IS NOT NULL OR stage IN ('committing','verifying','published') THEN sqlc.arg(commit_technical_detail)
      ELSE sqlc.arg(precommit_technical_detail)
    END,
    manifest_json=CASE
      WHEN committed_at IS NULL AND stage NOT IN ('committing','verifying','published')
        AND sqlc.arg(precommit_status)='needs_attention' THEN manifest_json
      ELSE NULL
    END,
    lease_token_hash=NULL,lease_expires_at=NULL,updated_at=sqlc.arg(updated_at)
WHERE publish_jobs.id=sqlc.arg(id) AND publish_jobs.user_id=sqlc.arg(user_id)
  AND publish_jobs.agent_id=sqlc.arg(agent_id) AND publish_jobs.status='running'
  AND publish_jobs.lease_token_hash=sqlc.arg(lease_token_hash)
  AND publish_jobs.lease_expires_at>sqlc.arg(now)
  AND publish_jobs.progress_seq<sqlc.arg(next_seq)
  AND EXISTS (
    SELECT 1 FROM publishing_agents AS active_agent
    WHERE active_agent.id=publish_jobs.agent_id
      AND active_agent.user_id=publish_jobs.user_id
      AND active_agent.revoked_at IS NULL
  );

-- name: CancelPublishJob :execrows
UPDATE publish_jobs
SET status='canceled',manifest_json=NULL,lease_token_hash=NULL,lease_expires_at=NULL,
    error_code=NULL,error_message=NULL,error_reason=NULL,error_params=NULL,technical_detail=NULL,updated_at=?
WHERE id=? AND user_id=? AND status IN ('queued','running','needs_attention')
  AND committed_at IS NULL AND stage NOT IN ('committing','verifying','published');

-- name: RequeueExpiredPreCommitJobs :execrows
UPDATE publish_jobs
SET status='queued',stage='queued',progress_seq=0,lease_token_hash=NULL,lease_expires_at=NULL,
    error_code=NULL,error_message=NULL,error_reason=NULL,error_params=NULL,technical_detail=NULL,updated_at=?
WHERE status='running' AND stage NOT IN ('committing','verifying','published') AND lease_expires_at<=?;

-- name: MarkExpiredCommittedJobsUnknown :execrows
UPDATE publish_jobs
SET status='outcome_unknown',manifest_json=NULL,lease_token_hash=NULL,lease_expires_at=NULL,
    error_code=NULL,error_message=NULL,error_reason=?,error_params=?,technical_detail=?,updated_at=?
WHERE status='running' AND stage IN ('committing','verifying') AND lease_expires_at<=?;

-- name: DeletePublishAssets :exec
DELETE FROM publish_assets WHERE job_id=?;

-- name: ListLiveStagedKeys :many
SELECT a.staged_key FROM publish_assets a
JOIN publish_jobs j ON j.id=a.job_id
WHERE j.status IN ('queued','running','needs_attention','outcome_unknown');

-- name: ListTerminalJobsWithAssets :many
SELECT DISTINCT j.id FROM publish_jobs j
JOIN publish_assets a ON a.job_id=j.id
WHERE j.status IN ('published','failed','outcome_unknown','canceled');
