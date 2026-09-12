-- name: InsertJob :exec
INSERT INTO generation_jobs (
    id, post_slug, user_id, voice_id, clip_project_id, dispatch_ready, cancellation_policy_version, kind, status, stage, progress_done, progress_total,
    error, observe_model, write_model, target_language, payload, created_at, updated_at,
    started_at, finished_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'queued', NULL, 0, 0, NULL, ?, ?, ?, ?, ?, ?, NULL, NULL);

-- name: PickNextQueued :one
UPDATE generation_jobs
SET status = 'running',
    stage = CASE kind
        WHEN 'generate_clip' THEN 'prepare'
        WHEN 'analyze_voice' THEN 'analyze'
        WHEN 'learn_voice' THEN 'learn'
        WHEN 'compare_voice_rule' THEN 'compare_rule'
        WHEN 'validate_voice_profile' THEN 'validate_profile'
        WHEN 'revise' THEN 'write'
        ELSE 'observe'
    END,
    error = NULL,
    error_reason = NULL,
    error_params = NULL,
    technical_detail = NULL,
    started_at = ?,
    updated_at = ?
WHERE id = (
    SELECT id FROM generation_jobs
    WHERE status = 'queued' AND dispatch_ready=1 AND cancel_requested_at IS NULL
    ORDER BY created_at, id
    LIMIT 1
)
RETURNING *;

-- name: UpdateProgress :exec
UPDATE generation_jobs
SET stage = ?, progress_done = ?, progress_total = ?, updated_at = ?
WHERE id = ? AND status = 'running' AND cancel_requested_at IS NULL;

-- name: FinishJob :execrows
UPDATE generation_jobs
SET status = CASE WHEN cancel_requested_at IS NOT NULL THEN 'cancelled' ELSE sqlc.arg(status) END,
    error = NULL,
    error_reason = CASE WHEN cancel_requested_at IS NULL THEN sqlc.narg(error_reason) ELSE NULL END,
    error_params = CASE WHEN cancel_requested_at IS NULL THEN sqlc.narg(error_params) ELSE NULL END,
    technical_detail = CASE WHEN cancel_requested_at IS NULL THEN sqlc.narg(technical_detail) ELSE NULL END,
    finished_at = sqlc.narg(finished_at), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND status = 'running';

-- name: FailQueuedJob :execrows
UPDATE generation_jobs
SET status = 'failed', error = NULL, error_reason = ?, error_params = ?, technical_detail = ?,
    finished_at = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND status = 'queued' AND cancel_requested_at IS NULL;

-- name: SweepRunning :execrows
UPDATE generation_jobs
SET status = 'failed', error = NULL, error_reason = ?, error_params = ?, technical_detail = ?,
    finished_at = ?, updated_at = ?
WHERE status = 'running' AND cancel_requested_at IS NULL;

-- name: SweepQueuedPersonalization :execrows
UPDATE generation_jobs
SET status = 'failed', error = NULL, error_reason = ?, error_params = ?, technical_detail = ?,
    finished_at = ?, updated_at = ?
WHERE status = 'queued'
  AND kind IN ('learn_voice', 'compare_voice_rule', 'validate_voice_profile', 'seed_voice');

-- name: ActiveForPost :one
SELECT * FROM generation_jobs
WHERE post_slug = ? AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ActiveForPostUser :one
SELECT * FROM generation_jobs
WHERE post_slug = ? AND user_id = ? AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ActiveForUserKind :one
SELECT * FROM generation_jobs
WHERE user_id = ? AND kind = ? AND post_slug IS NULL AND clip_project_id IS NULL
  AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ActiveForVoiceKind :one
-- Voice-owned work is guarded per voice: two voices may analyze at the same time, while one
-- voice still cannot run two of the same kind. Learning/comparison jobs may carry a post.
SELECT * FROM generation_jobs
WHERE voice_id = ? AND kind = ?
  AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ActiveForVoice :one
-- Anything queued or running that was frozen to this voice, post-backed or not: its result
-- would land in the voice, so a soft delete has to wait for it.
SELECT * FROM generation_jobs
WHERE voice_id = ? AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ActiveModelExperiment :one
SELECT * FROM generation_jobs
WHERE kind = 'model_experiment' AND payload = ? AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetJobByID :one
SELECT * FROM generation_jobs WHERE id = ?;

-- name: ActiveForClip :one
SELECT * FROM generation_jobs WHERE user_id=? AND clip_project_id=? AND status IN ('queued','running') LIMIT 1;
-- name: LatestForClip :one
SELECT * FROM generation_jobs WHERE user_id=? AND clip_project_id=? ORDER BY created_at DESC,id DESC LIMIT 1;
-- name: ActivateClip :execrows
UPDATE generation_jobs SET dispatch_ready=1 WHERE user_id=? AND id=? AND kind IN ('generate_clip','render_clip') AND status='queued' AND dispatch_ready=0 AND cancel_requested_at IS NULL;
-- name: SweepUnactivatedClips :execrows
UPDATE generation_jobs SET status='failed', error_reason=?, error_params=?, technical_detail=?, finished_at=?, updated_at=? WHERE kind IN ('generate_clip','render_clip') AND status='queued' AND dispatch_ready=0 AND cancel_requested_at IS NULL;

-- name: RequestClipCancellation :execrows
UPDATE generation_jobs SET cancel_requested_at=sqlc.arg(now), updated_at=sqlc.arg(now),
 status=CASE WHEN status='queued' THEN 'cancelled' ELSE status END,
 finished_at=CASE WHEN status='queued' THEN sqlc.arg(now) ELSE finished_at END
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND clip_project_id=sqlc.arg(project_id)
 AND status IN ('queued','running') AND cancel_requested_at IS NULL
 AND (kind='render_clip' OR (kind='generate_clip' AND cancellation_policy_version=1));
-- name: RecoverClipCancellations :execrows
UPDATE generation_jobs SET status='cancelled',finished_at=sqlc.arg(now),updated_at=sqlc.arg(now),
 error=NULL,error_reason=NULL,error_params=NULL,technical_detail=NULL
WHERE status IN ('queued','running') AND cancel_requested_at IS NOT NULL
 AND kind IN ('generate_clip','render_clip');
-- name: AuthorizeClipDispatch :execrows
UPDATE generation_jobs SET updated_at=updated_at
WHERE id=? AND user_id=? AND kind='generate_clip' AND status='running' AND cancel_requested_at IS NULL;
