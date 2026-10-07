-- name: PublishPostOriginContent :execrows
UPDATE posts SET input_revision = input_revision + CASE WHEN storyline IS NOT sqlc.narg(storyline) THEN 1 ELSE 0 END,
    content = sqlc.arg(content), machine_baseline = sqlc.arg(content),
    content_language = sqlc.arg(content_language), content_nouns = sqlc.narg(content_nouns),
    storyline = sqlc.narg(storyline), storyline_origins = sqlc.narg(storyline_origins),
    content_origins = sqlc.narg(content_origins),
    content_revision = content_revision + 1, machine_baseline_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id)
    AND content_revision = sqlc.arg(expected_content_revision) AND status <> 'published';

-- name: PublishPostOriginStoryline :execrows
UPDATE posts SET input_revision = input_revision + CASE WHEN storyline IS NOT sqlc.narg(storyline) THEN 1 ELSE 0 END,
    storyline = sqlc.narg(storyline), storyline_origins = sqlc.narg(storyline_origins), updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id)
    AND content_revision = sqlc.arg(expected_content_revision)
    AND input_revision = sqlc.arg(expected_input_revision) AND status <> 'published';

-- name: SetPostOriginAvailability :execrows
UPDATE posts SET content_origins = sqlc.narg(content_origins), storyline_origins = sqlc.narg(storyline_origins)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id) AND status <> 'published';
