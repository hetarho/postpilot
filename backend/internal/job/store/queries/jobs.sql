-- name: InsertJob :exec
INSERT INTO generation_jobs (
    id, post_slug, user_id, kind, status, stage, progress_done, progress_total,
    error, observe_model, write_model, payload, created_at, updated_at,
    started_at, finished_at
) VALUES (?, ?, ?, ?, 'queued', NULL, 0, 0, NULL, ?, ?, ?, ?, ?, NULL, NULL);

-- name: PickNextQueued :one
UPDATE generation_jobs
SET status = 'running',
    stage = CASE kind
        WHEN 'analyze_voice' THEN 'analyze'
        WHEN 'learn_voice' THEN 'learn'
        WHEN 'compare_voice_rule' THEN 'compare_rule'
        WHEN 'validate_voice_profile' THEN 'validate_profile'
        WHEN 'revise' THEN 'write'
        ELSE 'observe'
    END,
    started_at = ?,
    updated_at = ?
WHERE id = (
    SELECT id FROM generation_jobs
    WHERE status = 'queued'
    ORDER BY created_at, id
    LIMIT 1
)
RETURNING *;

-- name: UpdateProgress :exec
UPDATE generation_jobs
SET stage = ?, progress_done = ?, progress_total = ?, updated_at = ?
WHERE id = ? AND status = 'running';

-- name: FinishJob :exec
UPDATE generation_jobs
SET status = ?, error = ?, finished_at = ?, updated_at = ?
WHERE id = ? AND status = 'running';

-- name: FailQueuedJob :execrows
UPDATE generation_jobs
SET status = 'failed', error = ?, finished_at = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND status = 'queued';

-- name: SweepRunning :execrows
UPDATE generation_jobs
SET status = 'failed', error = ?, finished_at = ?, updated_at = ?
WHERE status = 'running';

-- name: SweepQueuedPersonalization :execrows
UPDATE generation_jobs
SET status = 'failed', error = ?, finished_at = ?, updated_at = ?
WHERE status = 'queued'
  AND kind IN ('learn_voice', 'compare_voice_rule', 'validate_voice_profile');

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
WHERE user_id = ? AND kind = ? AND post_slug IS NULL
  AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ActiveModelExperiment :one
SELECT * FROM generation_jobs
WHERE kind = 'model_experiment' AND payload = ? AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetJobByID :one
SELECT * FROM generation_jobs WHERE id = ?;
