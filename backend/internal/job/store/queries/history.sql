-- Generic bulk projections. Caller-owned kind filters; latest success replaces old failure.
-- name: LatestPostJobs :many
WITH ranked AS (
 SELECT g.id,ROW_NUMBER() OVER(PARTITION BY g.post_slug ORDER BY g.created_at DESC,g.id DESC) AS position
 FROM generation_jobs g
 WHERE g.user_id=sqlc.arg(user_id)
   AND (sqlc.arg(kinds)='[]' OR g.kind IN (SELECT value FROM json_each(sqlc.arg(kinds))))
   AND g.post_slug IN (SELECT value FROM json_each(sqlc.arg(subject_ids)))
)
SELECT j.* FROM generation_jobs j JOIN ranked r ON r.id=j.id WHERE r.position=1;

-- name: LatestVoiceJobs :many
WITH ranked AS (
 SELECT g.id,ROW_NUMBER() OVER(PARTITION BY g.voice_id ORDER BY g.created_at DESC,g.id DESC) AS position
 FROM generation_jobs g
 WHERE g.user_id=sqlc.arg(user_id)
   AND (sqlc.arg(kinds)='[]' OR g.kind IN (SELECT value FROM json_each(sqlc.arg(kinds))))
   AND g.voice_id IN (SELECT value FROM json_each(sqlc.arg(subject_ids)))
)
SELECT j.* FROM generation_jobs j JOIN ranked r ON r.id=j.id WHERE r.position=1;

-- name: LatestProjectJobs :many
WITH ranked AS (
 SELECT g.id,ROW_NUMBER() OVER(PARTITION BY g.clip_project_id ORDER BY g.created_at DESC,g.id DESC) AS position
 FROM generation_jobs g
 WHERE g.user_id=sqlc.arg(user_id)
   AND (sqlc.arg(kinds)='[]' OR g.kind IN (SELECT value FROM json_each(sqlc.arg(kinds))))
   AND g.clip_project_id IN (SELECT value FROM json_each(sqlc.arg(subject_ids)))
)
SELECT j.* FROM generation_jobs j JOIN ranked r ON r.id=j.id WHERE r.position=1;

-- name: LatestExperimentJobs :many
WITH ranked AS (
 SELECT g.id,ROW_NUMBER() OVER(PARTITION BY g.experiment_id ORDER BY g.created_at DESC,g.id DESC) AS position
 FROM generation_jobs g
 WHERE g.user_id=sqlc.arg(user_id)
   AND (sqlc.arg(kinds)='[]' OR g.kind IN (SELECT value FROM json_each(sqlc.arg(kinds))))
   AND g.experiment_id IN (SELECT value FROM json_each(sqlc.arg(subject_ids)))
)
SELECT j.* FROM generation_jobs j JOIN ranked r ON r.id=j.id WHERE r.position=1;
