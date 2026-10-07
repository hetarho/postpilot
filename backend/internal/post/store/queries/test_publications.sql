-- Post-owned winner application and receipt. No test or job tables are read here.
-- name: GetPostTestPublications :many
SELECT fingerprint, receipt FROM post_test_publications
WHERE user_id = sqlc.arg(user_id)
  AND ((test_id = sqlc.arg(test_id) AND winner_candidate_id = sqlc.arg(winner_candidate_id) AND action = 'apply_output')
       OR request_key = sqlc.arg(request_key));

-- name: ApplyPostTestOutput :execrows
UPDATE posts SET content = sqlc.arg(content), machine_baseline = sqlc.arg(machine_baseline),
    content_language = sqlc.arg(content_language), content_nouns = sqlc.narg(content_nouns),
    storyline = sqlc.narg(storyline),
    input_revision = input_revision + CASE WHEN storyline IS NOT sqlc.narg(storyline) THEN 1 ELSE 0 END,
    content_revision = content_revision + 1, machine_baseline_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id)
  AND input_revision = sqlc.arg(expected_input_revision)
  AND content_revision = sqlc.arg(expected_content_revision)
  AND status IN ('draft', 'review') AND status <> 'published';

-- name: CreatePostTestPublication :exec
INSERT INTO post_test_publications
    (user_id, test_id, winner_candidate_id, action, request_key, fingerprint,
     post_slug, resulting_content_revision, receipt, created_at)
VALUES (sqlc.arg(user_id), sqlc.arg(test_id), sqlc.arg(winner_candidate_id), 'apply_output',
    sqlc.arg(request_key), sqlc.arg(fingerprint), sqlc.arg(post_slug),
    sqlc.arg(resulting_content_revision), sqlc.arg(receipt), sqlc.arg(created_at));
