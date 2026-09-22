-- name: InsertExperiment :exec
INSERT INTO model_experiments (
  id, user_id, post_slug, voice_id, template_name, target_language, stage, origin, status, job_id, input_snapshot, input_hash,
  prompt_version, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertCandidate :exec
INSERT INTO model_experiment_candidates (
  id, experiment_id, model_provider_id, model_id, model_label, display_side, status
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteExperiment :exec
DELETE FROM model_experiments WHERE id = ?;

-- name: SetExperimentJob :exec
UPDATE model_experiments SET job_id = ? WHERE id = ? AND user_id = ?;

-- name: SetExperimentSnapshot :exec
UPDATE model_experiments
SET input_snapshot = ?, input_hash = ?, prompt_version = ?
WHERE id = ?;

-- name: GetExperiment :one
SELECT * FROM model_experiments WHERE id = ?;

-- name: GetExperimentForUser :one
SELECT * FROM model_experiments WHERE id = ? AND user_id = ?;

-- name: ListCandidates :many
SELECT * FROM model_experiment_candidates WHERE experiment_id = ? ORDER BY display_side;

-- name: ListExperimentsForUser :many
SELECT * FROM model_experiments
WHERE user_id = ? AND (? = '' OR stage = ?)
ORDER BY created_at DESC, id DESC;

-- name: PendingWriteForPost :one
SELECT * FROM model_experiments
WHERE user_id = ? AND post_slug = ? AND stage = 'write'
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
	OR (status = 'decided' AND (
	      (apply_requested = 1 AND applied_at IS NULL)
	      OR (adoption_requested = 1 AND adopted_at IS NULL)
	   ))
  )
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: CountPublishableForVoice :one
-- An experiment frozen to the voice that could still publish into it (a styleguide for an
-- analyze comparison, a machine baseline for a write one): unfinished, awaiting a verdict, or
-- decided with its winner not yet applied. ASCII only: sqlc expands SELECT * by byte offset
-- and a multi-byte character in this file corrupts the queries after it.
SELECT count(*) FROM model_experiments
WHERE voice_id = ? AND user_id = ?
  AND (status IN ('queued', 'running', 'review', 'partial')
       OR (status = 'decided' AND applied_at IS NULL));

-- name: SetExperimentStatus :exec
UPDATE model_experiments
SET status = ?, finished_at = COALESCE(?, finished_at)
WHERE id = ?;

-- name: StartCandidate :execrows
UPDATE model_experiment_candidates
SET status = 'running', error = NULL, error_reason = NULL, error_params = NULL,
    technical_detail = NULL, started_at = ?, finished_at = NULL
WHERE id = ? AND experiment_id = ? AND status IN ('pending', 'failed');

-- name: CompleteCandidate :execrows
UPDATE model_experiment_candidates
SET status = ?, output = ?, error = NULL, error_reason = ?, error_params = ?,
    technical_detail = ?, prompt_tokens = ?, completion_tokens = ?, cost_microusd = ?,
    cost_source = ?, latency_ms = ?, finished_at = ?
WHERE id = ? AND experiment_id = ?;

-- name: ListInterruptedExperimentIDs :many
SELECT id FROM model_experiments WHERE status = 'running' ORDER BY created_at, id;

-- name: ListQueuedExperimentIDs :many
SELECT id FROM model_experiments WHERE status = 'queued' ORDER BY created_at, id;

-- name: FailUnfinishedCandidates :execrows
UPDATE model_experiment_candidates
SET status = 'failed', error = NULL, error_reason = ?, error_params = ?,
    technical_detail = ?, finished_at = ?
WHERE experiment_id = ? AND status IN ('pending', 'running');

-- name: FinishInterruptedExperiment :execrows
UPDATE model_experiments
SET status = CASE
      WHEN EXISTS (
        SELECT 1 FROM model_experiment_candidates
        WHERE experiment_id = model_experiments.id AND status = 'succeeded'
      ) THEN 'partial'
      ELSE 'failed'
    END,
    finished_at = ?
WHERE model_experiments.id = ? AND model_experiments.status IN ('queued', 'running');

-- name: ResetFailedCandidates :execrows
UPDATE model_experiment_candidates
SET status = 'pending', error = NULL, error_reason = NULL, error_params = NULL,
    technical_detail = NULL, started_at = NULL, finished_at = NULL
WHERE experiment_id = ? AND status = 'failed';

-- name: RestoreFailedCandidate :execrows
UPDATE model_experiment_candidates
SET status = 'failed', error = NULL, error_reason = ?, error_params = ?,
    technical_detail = ?, started_at = ?, finished_at = ?
WHERE experiment_id = ? AND id = ? AND status = 'pending';

-- name: DecideExperiment :execrows
UPDATE model_experiments
SET status = ?, winner_candidate_id = ?, outcome = ?, decided_at = ?,
    content_expires_at = ?, apply_error = NULL, apply_error_reason = NULL,
    apply_error_params = NULL, apply_technical_detail = NULL, applied_at = NULL,
	apply_requested = ?, adoption_requested = ?, adoption_error = NULL, adoption_error_reason = NULL,
    adoption_error_params = NULL, adoption_technical_detail = NULL, adopted_at = NULL
WHERE id = ? AND user_id = ?
  AND status IN ('review', 'partial', 'failed');

-- name: InsertVerdictBadge :exec
-- Written in the same transaction as the verdict it explains. A repeat of the same badge for
-- the same candidate is the same row, so a retried write changes nothing.
INSERT INTO model_experiment_badges (experiment_id, candidate_id, badge, note)
VALUES (?, ?, ?, ?)
ON CONFLICT (experiment_id, candidate_id, badge) DO NOTHING;

-- name: ClearVerdictBadges :exec
-- A verdict may be recorded again on the same comparison (an application retry replays it),
-- so the badge rows are replaced with the verdict rather than accumulated across attempts.
DELETE FROM model_experiment_badges WHERE experiment_id = ?;

-- name: ListVerdictBadges :many
SELECT * FROM model_experiment_badges
WHERE experiment_id = ?
ORDER BY candidate_id, badge;

-- name: PurgeExpiredBadgeNotes :exec
-- The free note is private payload and leaves with the rest of it; the badge ids stay, so a
-- leaderboard can still tally what a model earned (MODEL-42).
UPDATE model_experiment_badges SET note = NULL
WHERE experiment_id IN (
  SELECT id FROM model_experiments
  WHERE content_expires_at IS NOT NULL AND content_expires_at <= ?
    AND status IN ('decided', 'dismissed')
);

-- name: PurgePostBadgeNotes :exec
UPDATE model_experiment_badges SET note = NULL
WHERE experiment_id IN (SELECT id FROM model_experiments WHERE user_id = ? AND post_slug = ?);

-- name: SetApplyRequested :execrows
-- Marks that this decided verdict now owes a content application, so a failure leaves the
-- comparison unresolved for its post with a visible retry. Idempotent: a repeat is a no-op.
UPDATE model_experiments SET apply_requested = 1
WHERE id = ? AND user_id = ? AND status = 'decided' AND applied_at IS NULL;

-- name: SetApplyFailure :exec
UPDATE model_experiments
SET apply_error = NULL, apply_error_reason = ?, apply_error_params = ?,
    apply_technical_detail = ?, applied_at = NULL
WHERE id = ? AND user_id = ?;

-- name: SetExperimentApplied :execrows
UPDATE model_experiments
SET apply_error = NULL, apply_error_reason = NULL, apply_error_params = NULL,
    apply_technical_detail = NULL, applied_at = ?
WHERE id = ? AND user_id = ? AND status = 'decided' AND applied_at IS NULL;

-- name: SetAdoptionFailure :exec
UPDATE model_experiments
SET adoption_error = NULL, adoption_error_reason = ?, adoption_error_params = ?,
    adoption_technical_detail = ?
WHERE id = ? AND user_id = ? AND adoption_requested = 1 AND adopted_at IS NULL;

-- name: SetExperimentAdopted :execrows
UPDATE model_experiments
SET adoption_error = NULL, adoption_error_reason = NULL, adoption_error_params = NULL,
    adoption_technical_detail = NULL, adopted_at = ?
WHERE id = ? AND user_id = ? AND status = 'decided'
  AND adoption_requested = 1 AND adopted_at IS NULL;

-- name: PurgeExpiredContent :execrows
UPDATE model_experiments
SET input_snapshot = NULL
WHERE content_expires_at IS NOT NULL AND content_expires_at <= ?
  AND status IN ('decided', 'dismissed') AND input_snapshot IS NOT NULL;

-- name: PurgeExpiredCandidateOutput :execrows
UPDATE model_experiment_candidates
SET output = NULL
WHERE experiment_id IN (
  SELECT id FROM model_experiments
  WHERE content_expires_at IS NOT NULL AND content_expires_at <= ?
    AND status IN ('decided', 'dismissed')
);

-- name: PurgePostContent :exec
UPDATE model_experiments SET input_snapshot = NULL WHERE user_id = ? AND post_slug = ?;

-- name: PurgePostCandidateOutput :exec
UPDATE model_experiment_candidates SET output = NULL
WHERE experiment_id IN (SELECT id FROM model_experiments WHERE user_id = ? AND post_slug = ?);

-- name: ListDecidedForLeaderboard :many
-- The winner verdicts one account reached inside the window. decided_at is stored in a
-- fixed-width UTC layout, so the string comparison is the chronological one.
SELECT * FROM model_experiments
WHERE user_id = ? AND stage = ? AND outcome = 'winner' AND winner_candidate_id IS NOT NULL
  AND decided_at IS NOT NULL AND decided_at >= ?
ORDER BY decided_at, id;

-- name: ListDecidedForLeaderboardAll :many
-- The same, over every account. Only the candidates' model refs leave this query, so no
-- account, experiment or output reaches the board it feeds.
SELECT * FROM model_experiments
WHERE stage = ? AND outcome = 'winner' AND winner_candidate_id IS NOT NULL
  AND decided_at IS NOT NULL AND decided_at >= ?
ORDER BY decided_at, id;

-- name: ListCandidatesForLeaderboard :many
-- The call accounting beside those verdicts, from the comparisons that were resolved inside
-- the same window: a comparison still awaiting a verdict has not earned a place on a board.
SELECT c.* FROM model_experiment_candidates c
JOIN model_experiments e ON e.id = c.experiment_id
WHERE e.user_id = ? AND e.stage = ?
  AND e.status IN ('decided', 'dismissed')
  AND e.decided_at IS NOT NULL AND e.decided_at >= ?
ORDER BY e.decided_at, e.id, c.display_side;

-- name: ListBadgeTalliesForLeaderboard :many
-- How often each model earned each badge inside this board's window. Grouped by the model a
-- candidate ran, not by the candidate: a board ranks models, and two comparisons of the same
-- model are the same row here. The note is deliberately not selected.
SELECT c.model_provider_id, c.model_id, b.badge, count(*) AS total
FROM model_experiment_badges b
JOIN model_experiment_candidates c ON c.experiment_id = b.experiment_id AND c.id = b.candidate_id
JOIN model_experiments e ON e.id = b.experiment_id
WHERE e.user_id = ? AND e.stage = ? AND e.outcome = 'winner'
  AND e.decided_at IS NOT NULL AND e.decided_at >= ?
GROUP BY c.model_provider_id, c.model_id, b.badge;

-- name: ListBadgeTalliesForLeaderboardAll :many
SELECT c.model_provider_id, c.model_id, b.badge, count(*) AS total
FROM model_experiment_badges b
JOIN model_experiment_candidates c ON c.experiment_id = b.experiment_id AND c.id = b.candidate_id
JOIN model_experiments e ON e.id = b.experiment_id
WHERE e.stage = ? AND e.outcome = 'winner'
  AND e.decided_at IS NOT NULL AND e.decided_at >= ?
GROUP BY c.model_provider_id, c.model_id, b.badge;

-- name: ListCandidatesForLeaderboardAll :many
SELECT c.* FROM model_experiment_candidates c
JOIN model_experiments e ON e.id = c.experiment_id
WHERE e.stage = ?
  AND e.status IN ('decided', 'dismissed')
  AND e.decided_at IS NOT NULL AND e.decided_at >= ?
ORDER BY e.decided_at, e.id, c.display_side;
