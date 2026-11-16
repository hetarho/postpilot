-- name: ListClipUploadTexts :many
SELECT * FROM clip_upload_texts WHERE project_id=? AND user_id=?;
-- name: GetClipUploadText :one
SELECT * FROM clip_upload_texts WHERE project_id=? AND user_id=?;
-- name: InsertClipUploadText :execrows
INSERT INTO clip_upload_texts(project_id,user_id,body,tags,edited,updated_at,revision,last_job_id) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(project_id) DO NOTHING;
-- name: UpdateClipUploadText :execrows
UPDATE clip_upload_texts SET body=?,tags=?,edited=?,updated_at=?,revision=revision+1,last_job_id=? WHERE project_id=? AND user_id=? AND revision=?;
-- name: SaveClipUploadTextQuote :execrows
INSERT INTO clip_upload_text_quotes(id,user_id,project_id,expected_revision,input_digest,pricing_json,max_credits,expires_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(project_id) WHERE consumed_job_id IS NULL DO UPDATE SET id=excluded.id,expected_revision=excluded.expected_revision,input_digest=excluded.input_digest,pricing_json=excluded.pricing_json,max_credits=excluded.max_credits,expires_at=excluded.expires_at
WHERE clip_upload_text_quotes.consumed_job_id IS NULL AND clip_upload_text_quotes.user_id=excluded.user_id;
-- name: GetClipUploadTextQuote :one
SELECT * FROM clip_upload_text_quotes WHERE user_id=? AND id=?;
-- name: ConsumeClipUploadTextQuote :execrows
UPDATE clip_upload_text_quotes SET consumed_job_id=? WHERE user_id=? AND id=? AND consumed_job_id IS NULL AND expires_at>?;
