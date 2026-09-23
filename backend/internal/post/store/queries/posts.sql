-- Posts. Every query is scoped by user_id: ownership is enforced in SQL, not by a
-- caller remembering to check it.
--
-- A published post is locked (POST-74): every write to it but the address's and the delete
-- carries `status <> 'published'`, so a write that passed the service's check and then lost
-- the race to a publish matches zero rows instead of rewriting a post that is already live.

-- name: CreatePost :exec
-- target_length and tag_count are the template's seeds when the create names one, and NULL
-- otherwise: a post nobody gave a number to reads as natural length and the default count.
-- field is the blog field the create named, NULL for none.
INSERT INTO posts (slug, user_id, voice_id, template_id, field, title, memo, target_language,
    target_length, tag_count, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?);

-- name: UpdatePostDraft :execrows
UPDATE posts SET title = sqlc.arg(title), memo = sqlc.arg(memo),
    target_language = COALESCE(sqlc.narg(target_language), target_language),
    updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id) AND status <> 'published';

-- name: UpdatePostObservations :execrows
UPDATE posts SET observations = ?, updated_at = ?
WHERE slug = ? AND user_id = ? AND status <> 'published';

-- name: UpdateGeneratedContent :execrows
-- The write's nouns and replacement candidates ride the same statement, beside the content and
-- never inside it (GEN-53, GEN-55). The service resolves them first, so NULL here always means
-- none, and an identical content with different ones is a new machine write.
UPDATE posts SET content = sqlc.arg(content), machine_baseline = sqlc.arg(machine_baseline), machine_baseline_voice_id = voice_id,
    content_language = sqlc.arg(content_language),
    content_nouns = sqlc.narg(content_nouns),
    replacement_candidates = sqlc.narg(replacement_candidates),
    content_revision = content_revision + 1,
    machine_baseline_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id) AND status <> 'published'
  AND (content IS NULL OR content <> sqlc.arg(content) OR status <> 'review'
       OR machine_baseline_revision <> content_revision
       OR content_language IS NULL OR content_language <> sqlc.arg(content_language)
       OR content_nouns IS NOT sqlc.narg(content_nouns)
       OR replacement_candidates IS NOT sqlc.narg(replacement_candidates));

-- name: SavePostContent :execrows
UPDATE posts SET content = ?, content_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = ?
WHERE slug = ? AND user_id = ? AND content_revision = ? AND status <> 'published';

-- name: SavePostGenerationOptions :execrows
-- use_memory and the quality ticks ride this save rather than the draft's: they are options of
-- the RUN, and like the two numbers beside them they change no status, revision or baseline
-- (MEM-18, POST-82). quality_rules is JSON, NULL for none.
UPDATE posts SET target_length = ?, tag_count = ?, use_memory = ?, quality_rules = ?, updated_at = ?
WHERE slug = ? AND user_id = ? AND status <> 'published';

-- Finalizing also copies the confirmed AI title into posts.title (spec/legacy/policy/posts.md). ONE
-- statement, still guarded by the exact revision, so the copy is atomic with the finalization and
-- a concurrent content save simply matches zero rows. The caller resolves which title to write: an
-- empty content title leaves the user's working title in place. The slug is never re-minted.
-- name: FinalizePost :execrows
UPDATE posts SET status = 'finalized', finalized_revision = content_revision,
    title = ?, finalized_at = ?, updated_at = ?
WHERE slug = ? AND user_id = ? AND content_revision = ?
  AND content IS NOT NULL AND status <> 'published';

-- name: GetPost :one
SELECT slug, user_id, voice_id, title, memo, observations, content, status, created_at, updated_at,
       content_revision, machine_baseline, machine_baseline_revision, machine_baseline_voice_id,
       target_length, finalized_revision, finalized_at, template_id, target_language, content_language,
       tag_count, use_memory, published_url, published_at, field, content_nouns, replacement_candidates,
       quality_rules
FROM posts WHERE slug = ?;

-- name: PublishPost :execrows
-- Records or replaces the Naver Blog address. Only a post whose current revision is its
-- finalized one, or one already published, can take it (POST-73, POST-75).
UPDATE posts SET status = 'published', published_url = ?, published_at = ?, updated_at = ?
WHERE slug = ? AND user_id = ?
  AND ((status = 'finalized' AND finalized_revision = content_revision) OR status = 'published');

-- name: UnpublishPost :execrows
-- Clearing the address returns the post to finalized; the finalization itself is untouched.
UPDATE posts SET status = 'finalized', published_url = NULL, published_at = NULL, updated_at = ?
WHERE slug = ? AND user_id = ? AND status = 'published';

-- name: ListPublishedPostsByUser :many
-- The account's published window, newest publication first (QUAL-39). Repeating the status in
-- the WHERE is what lets SQLite use the partial index posts_user_published_idx.
SELECT slug, content, content_language, content_revision, content_nouns, published_at
FROM posts WHERE user_id = ? AND status = 'published'
ORDER BY published_at DESC, slug DESC LIMIT ?;

-- name: GetLearningSnapshot :one
SELECT slug, user_id, voice_id, content, content_revision, machine_baseline, machine_baseline_revision,
       machine_baseline_voice_id, target_length, status, finalized_revision, finalized_at, updated_at,
       target_language, content_language
FROM posts WHERE slug = ? AND user_id = ?;

-- name: PostSlugExists :one
SELECT EXISTS (SELECT 1 FROM posts WHERE slug = ?);

-- name: PostIsPublished :one
-- The guard an insert under a post runs first in its own write transaction. It is a guard and
-- not an INSERT ... SELECT, which would insert nothing for an unknown post too: zero rows would
-- then say two things, and the composite foreign key's refusal would be lost.
SELECT EXISTS (SELECT 1 FROM posts WHERE slug = ? AND status = 'published');

-- name: ListPostsByUser :many
SELECT slug, title, content, status, updated_at, voice_id, template_id, target_language, content_language
FROM posts WHERE user_id = ? ORDER BY updated_at DESC, slug DESC;

-- name: ReassignPostVoice :execrows
-- The reassignment keeps every byte of the post and drops only what belonged to the old
-- voice: the machine baseline, and with it the eligibility to learn from this post until a
-- fresh machine result is written under the new voice.
UPDATE posts SET voice_id = ?, machine_baseline = NULL, machine_baseline_revision = 0,
    machine_baseline_voice_id = NULL,
    updated_at = ?
WHERE slug = ? AND user_id = ? AND voice_id <> ? AND status <> 'published';

-- name: AssignPostTemplate :execrows
-- Assignment is not a reassignment: unlike the voice, a template is never learned from, so
-- this touches no content, revision, machine baseline or finalization column and is allowed
-- in every status but published, which is locked. NULL is the clear.
--
-- It also SEEDS the two generation options from the template that is being assigned
-- (TEMPLATE-48): a seed parameter is non-NULL only for a number that template has set, so
-- COALESCE says exactly "overwrite when the template has an opinion, keep the post's own
-- otherwise", in this one statement, so a post can never be left seeded by an assignment
-- that did not land. Clearing the assignment passes no seed at all.
UPDATE posts SET template_id = sqlc.narg(template_id),
    target_length = COALESCE(sqlc.narg(seed_target_length), target_length),
    tag_count = COALESCE(sqlc.narg(seed_tag_count), tag_count),
    updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id) AND status <> 'published';

-- name: AssignPostField :execrows
-- The blog field, NULL for none. Like the template it touches no status, revision, baseline or
-- finalization column (POST-82). An equal value matches no row: IS NOT is SQLite's NULL-safe
-- inequality, so clearing a field that is already none writes nothing either.
UPDATE posts SET field = sqlc.narg(field), updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id) AND status <> 'published'
  AND field IS NOT sqlc.narg(field);

-- name: CountPostsByVoice :one
SELECT count(*) FROM posts WHERE voice_id = ? AND user_id = ?;

-- name: DeletePost :execrows
DELETE FROM posts WHERE slug = ? AND user_id = ?;
