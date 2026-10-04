-- ASCII only: sqlc expands SELECT * by byte offset and a multi-byte character in this
-- file corrupts the queries after it.

-- name: InsertExperiment :exec
INSERT INTO model_experiments (
  id, user_id, post_slug, voice_id, template_name, target_language, stage, origin, status, job_id, input_snapshot, input_hash,
  prompt_version, created_at, source, voice_prompt_key, voice_material_id, review_mode
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

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
SELECT * FROM model_experiment_candidates WHERE experiment_id = ?
ORDER BY CASE display_side WHEN 'left' THEN 1 WHEN 'right' THEN 2
  WHEN 'c' THEN 3 WHEN 'd' THEN 4 WHEN 'e' THEN 5 ELSE 6 END;

-- name: ListExperimentSummariesForUser :many
-- An empty stage or source matches every one: the write history reads post-sourced comparisons
-- and the voice history voice-sourced ones (MODEL-67). A list row is a summary: the frozen
-- input is never read here, and it comes back as NULL only so the row keeps the table's shape.
-- No LIMIT: the posts page maps any pending comparison id to its status, however old.
SELECT
  id, user_id, post_slug, voice_id, stage, status, job_id,
  CAST(NULL AS TEXT) AS input_snapshot,
  input_hash, prompt_version, winner_candidate_id, outcome, apply_error, applied_at, created_at,
  finished_at, decided_at, content_expires_at, adoption_error, adopted_at, adoption_requested,
  template_name, target_language, apply_error_reason, apply_error_params, apply_technical_detail,
  adoption_error_reason, adoption_error_params, adoption_technical_detail, origin, apply_requested,
  source, voice_prompt_key, voice_material_id, review_mode, completed_at, applied_candidate_id,
  adopted_candidate_id
FROM model_experiments
WHERE user_id = sqlc.arg(user_id)
  AND (CAST(sqlc.arg(stage) AS TEXT) = '' OR stage = CAST(sqlc.arg(stage) AS TEXT))
  AND (CAST(sqlc.arg(source) AS TEXT) = '' OR source = CAST(sqlc.arg(source) AS TEXT))
ORDER BY created_at DESC, id DESC;

-- name: ListCandidateSummariesForExperiments :many
-- Every listed comparison's candidates in one read, without their output: a list row never
-- shows a candidate's work. ids is a JSON array of experiment ids.
SELECT
  id, experiment_id, model_provider_id, model_id, model_label, display_side, status,
  CAST(NULL AS TEXT) AS output,
  error, prompt_tokens, completion_tokens, cost_microusd, cost_source, latency_ms, started_at,
  finished_at, error_reason, error_params, technical_detail, rank
FROM model_experiment_candidates
WHERE experiment_id IN (SELECT value FROM json_each(sqlc.arg(ids)))
ORDER BY experiment_id, CASE display_side WHEN 'left' THEN 1 WHEN 'right' THEN 2
  WHEN 'c' THEN 3 WHEN 'd' THEN 4 WHEN 'e' THEN 5 ELSE 6 END;

-- name: ListVerdictBadgesForExperiments :many
-- Every listed comparison's badges in one read. ids is a JSON array of experiment ids.
SELECT * FROM model_experiment_badges
WHERE experiment_id IN (SELECT value FROM json_each(sqlc.arg(ids)))
ORDER BY experiment_id, candidate_id, badge;

-- name: PendingWriteForPost :one
SELECT * FROM model_experiments
WHERE user_id = ? AND post_slug = ? AND stage = 'write'
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
	OR (status = 'completed' AND origin = 'editor' AND applied_at IS NULL)
	OR (status IN ('decided', 'completed') AND (
	      (apply_requested = 1 AND applied_at IS NULL)
	      OR (adoption_requested = 1 AND adopted_at IS NULL)
	   ))
  )
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: BlockingWriteForPost :one
-- The editor write comparison that holds a post's generation and revision (GEN-23, GEN-38):
-- still queued, running, partial or in review, or decided with a requested application or
-- adoption not yet complete. A lab comparison and a failed one never hold the post.
SELECT id FROM model_experiments
WHERE user_id = ? AND post_slug = ? AND stage = 'write' AND origin = 'editor'
  AND (
    status IN ('queued', 'running', 'review', 'partial')
    OR (status = 'completed' AND applied_at IS NULL)
    OR (status IN ('decided', 'completed') AND (
          (apply_requested = 1 AND applied_at IS NULL)
          OR (adoption_requested = 1 AND adopted_at IS NULL)
       ))
  )
ORDER BY created_at DESC, id DESC LIMIT 1;

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

-- name: CompleteRankedExperiment :execrows
UPDATE model_experiments
SET status = 'completed', completed_at = ?, content_expires_at = ?
WHERE id = ? AND user_id = ? AND review_mode = 'candidate_ranking'
  AND status IN ('review', 'partial', 'failed');

-- name: SetCandidateRank :execrows
UPDATE model_experiment_candidates SET rank = ?
WHERE id = ? AND experiment_id = ? AND status = 'succeeded' AND rank IS NULL;

-- name: MarkCandidateApply :execrows
UPDATE model_experiments
SET applied_candidate_id = sqlc.arg(candidate_id), apply_requested = 1,
    adoption_requested = CASE WHEN sqlc.arg(adopt_model) = 1 THEN 1 ELSE adoption_requested END,
    adopted_candidate_id = CASE WHEN sqlc.arg(adopt_model) = 1 THEN sqlc.arg(candidate_id) ELSE adopted_candidate_id END
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND review_mode = 'candidate_ranking' AND status = 'completed'
  AND (applied_candidate_id IS NULL OR applied_candidate_id = sqlc.arg(candidate_id))
  AND (sqlc.arg(adopt_model) = 0 OR adopted_candidate_id IS NULL OR adopted_candidate_id = sqlc.arg(candidate_id));

-- name: MarkCandidateAdopt :execrows
UPDATE model_experiments SET adopted_candidate_id = sqlc.arg(candidate_id), adoption_requested = 1
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND review_mode = 'candidate_ranking' AND status = 'completed'
  AND (adopted_candidate_id IS NULL OR adopted_candidate_id = sqlc.arg(candidate_id));

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
    AND status IN ('decided', 'dismissed', 'completed')
);

-- name: PurgePostBadgeNotes :exec
UPDATE model_experiment_badges SET note = NULL
WHERE experiment_id IN (SELECT id FROM model_experiments WHERE user_id = ? AND post_slug = ?);

-- name: SetApplyRequested :execrows
-- Marks that this decided verdict now owes a content application, so a failure leaves the
-- comparison unresolved for its post with a visible retry. Idempotent: a repeat is a no-op.
UPDATE model_experiments SET apply_requested = 1
WHERE id = ? AND user_id = ? AND status = 'decided' AND applied_at IS NULL;

-- name: SetAdoptionRequested :execrows
-- Marks that this decided verdict now owes an active-model adoption, so the lab's follow-up
-- is reload-safe: a failure leaves a visible retry, and a success lands on adopted_at.
-- Idempotent: a repeat is a no-op.
UPDATE model_experiments SET adoption_requested = 1
WHERE id = ? AND user_id = ? AND status = 'decided' AND adopted_at IS NULL;

-- name: SetApplyFailure :exec
UPDATE model_experiments
SET apply_error = NULL, apply_error_reason = ?, apply_error_params = ?,
    apply_technical_detail = ?, applied_at = NULL
WHERE id = ? AND user_id = ?;

-- name: SetExperimentApplied :execrows
UPDATE model_experiments
SET apply_error = NULL, apply_error_reason = NULL, apply_error_params = NULL,
    apply_technical_detail = NULL, applied_at = ?
WHERE id = ? AND user_id = ? AND status IN ('decided', 'completed') AND applied_at IS NULL;

-- name: SetAdoptionFailure :exec
UPDATE model_experiments
SET adoption_error = NULL, adoption_error_reason = ?, adoption_error_params = ?,
    adoption_technical_detail = ?
WHERE id = ? AND user_id = ? AND adoption_requested = 1 AND adopted_at IS NULL;

-- name: SetExperimentAdopted :execrows
UPDATE model_experiments
SET adoption_error = NULL, adoption_error_reason = NULL, adoption_error_params = NULL,
    adoption_technical_detail = NULL, adopted_at = ?
WHERE id = ? AND user_id = ? AND status IN ('decided', 'completed')
  AND adoption_requested = 1 AND adopted_at IS NULL;

-- name: PurgeExpiredContent :execrows
UPDATE model_experiments
SET input_snapshot = NULL
WHERE content_expires_at IS NOT NULL AND content_expires_at <= ?
  AND status IN ('decided', 'dismissed', 'completed') AND input_snapshot IS NOT NULL;

-- name: PurgeExpiredCandidateOutput :execrows
UPDATE model_experiment_candidates
SET output = NULL
WHERE experiment_id IN (
  SELECT id FROM model_experiments
  WHERE content_expires_at IS NOT NULL AND content_expires_at <= ?
    AND status IN ('decided', 'dismissed', 'completed')
);

-- name: PurgePostContent :exec
UPDATE model_experiments SET input_snapshot = NULL WHERE user_id = ? AND post_slug = ?;

-- name: PurgePostCandidateOutput :exec
UPDATE model_experiment_candidates SET output = NULL
WHERE experiment_id IN (SELECT id FROM model_experiments WHERE user_id = ? AND post_slug = ?);

-- name: ListDecidedForLeaderboard :many
-- Ranked completions and eligible historical pairwise outcomes in the window. The service
-- keeps a legacy dismissal only when both candidates delivered. Both clocks are fixed-width UTC.
SELECT * FROM model_experiments
WHERE user_id = sqlc.arg(user_id) AND stage = sqlc.arg(stage)
  AND ((review_mode = 'pairwise' AND
       ((outcome = 'winner' AND winner_candidate_id IS NOT NULL)
        OR (status = 'dismissed' AND outcome = 'skipped')))
       OR (review_mode = 'candidate_ranking' AND status = 'completed' AND
           EXISTS (SELECT 1 FROM model_experiment_candidates c
                   WHERE c.experiment_id = model_experiments.id AND c.rank IS NOT NULL)))
  AND COALESCE(completed_at, decided_at) >= sqlc.arg(since)
ORDER BY COALESCE(completed_at, decided_at), id;

-- name: ListDecidedForLeaderboardAll :many
-- The same, over every account. The service projects only model-level figures to the RPC.
SELECT * FROM model_experiments
WHERE stage = sqlc.arg(stage)
  AND ((review_mode = 'pairwise' AND
       ((outcome = 'winner' AND winner_candidate_id IS NOT NULL)
        OR (status = 'dismissed' AND outcome = 'skipped')))
       OR (review_mode = 'candidate_ranking' AND status = 'completed' AND
           EXISTS (SELECT 1 FROM model_experiment_candidates c
                   WHERE c.experiment_id = model_experiments.id AND c.rank IS NOT NULL)))
  AND COALESCE(completed_at, decided_at) >= sqlc.arg(since)
ORDER BY COALESCE(completed_at, decided_at), id;

-- name: ListCandidatesForLeaderboard :many
-- Candidate accounting beside counted decisions; a blind or skipped run has earned no board row.
SELECT c.* FROM model_experiment_candidates c
JOIN model_experiments e ON e.id = c.experiment_id
WHERE e.user_id = sqlc.arg(user_id) AND e.stage = sqlc.arg(stage)
  AND ((e.review_mode = 'pairwise' AND e.status IN ('decided', 'dismissed'))
       OR (e.review_mode = 'candidate_ranking' AND e.status = 'completed' AND c.rank IS NOT NULL))
  AND COALESCE(e.completed_at, e.decided_at) >= sqlc.arg(since)
ORDER BY COALESCE(e.completed_at, e.decided_at), e.id,
  CASE c.display_side WHEN 'left' THEN 1 WHEN 'right' THEN 2 WHEN 'c' THEN 3 WHEN 'd' THEN 4 WHEN 'e' THEN 5 END;

-- name: ListBadgeTalliesForLeaderboard :many
-- How often each model earned each badge inside this board's window. Grouped by the model a
-- candidate ran, not by the candidate: a board ranks models, and two comparisons of the same
-- model are the same row here. The note is deliberately not selected.
SELECT c.model_provider_id, c.model_id, b.badge, count(*) AS total
FROM model_experiment_badges b
JOIN model_experiment_candidates c ON c.experiment_id = b.experiment_id AND c.id = b.candidate_id
JOIN model_experiments e ON e.id = b.experiment_id
WHERE e.user_id = sqlc.arg(user_id) AND e.stage = sqlc.arg(stage)
  AND ((e.review_mode = 'pairwise' AND e.outcome = 'winner')
       OR (e.review_mode = 'candidate_ranking' AND e.status = 'completed' AND c.rank IS NOT NULL))
  AND COALESCE(e.completed_at, e.decided_at) >= sqlc.arg(since)
GROUP BY c.model_provider_id, c.model_id, b.badge;

-- name: ListBadgeTalliesForLeaderboardAll :many
SELECT c.model_provider_id, c.model_id, b.badge, count(*) AS total
FROM model_experiment_badges b
JOIN model_experiment_candidates c ON c.experiment_id = b.experiment_id AND c.id = b.candidate_id
JOIN model_experiments e ON e.id = b.experiment_id
WHERE e.stage = sqlc.arg(stage)
  AND ((e.review_mode = 'pairwise' AND e.outcome = 'winner')
       OR (e.review_mode = 'candidate_ranking' AND e.status = 'completed' AND c.rank IS NOT NULL))
  AND COALESCE(e.completed_at, e.decided_at) >= sqlc.arg(since)
GROUP BY c.model_provider_id, c.model_id, b.badge;

-- name: ListCandidatesForLeaderboardAll :many
SELECT c.* FROM model_experiment_candidates c
JOIN model_experiments e ON e.id = c.experiment_id
WHERE e.stage = sqlc.arg(stage)
  AND ((e.review_mode = 'pairwise' AND e.status IN ('decided', 'dismissed'))
       OR (e.review_mode = 'candidate_ranking' AND e.status = 'completed' AND c.rank IS NOT NULL))
  AND COALESCE(e.completed_at, e.decided_at) >= sqlc.arg(since)
ORDER BY COALESCE(e.completed_at, e.decided_at), e.id,
  CASE c.display_side WHEN 'left' THEN 1 WHEN 'right' THEN 2 WHEN 'c' THEN 3 WHEN 'd' THEN 4 WHEN 'e' THEN 5 END;
