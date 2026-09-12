-- name: SaveClipQuote :execrows
INSERT INTO clip_generation_quotes(id,user_id,project_id,batch_id,input_digest,pricing_json,max_credits,expires_at)
VALUES (?,?,?,?,?,?,?,?)
ON CONFLICT(batch_id) WHERE consumed_job_id IS NULL DO UPDATE SET id=excluded.id,input_digest=excluded.input_digest,pricing_json=excluded.pricing_json,max_credits=excluded.max_credits,expires_at=excluded.expires_at
WHERE clip_generation_quotes.consumed_job_id IS NULL AND clip_generation_quotes.user_id=excluded.user_id AND clip_generation_quotes.project_id=excluded.project_id;

-- name: GetClipQuote :one
SELECT * FROM clip_generation_quotes WHERE user_id=? AND id=?;

-- name: ConsumeClipQuote :execrows
UPDATE clip_generation_quotes SET consumed_job_id=? WHERE user_id=? AND id=? AND consumed_job_id IS NULL AND expires_at>?;
