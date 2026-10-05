-- name: InsertClipSpeechAsset :execrows
INSERT INTO clip_speech_assets(id,owner_id,project_id,object_key,input_text,input_hash,binding_digest,speech_json,created_at)
SELECT ?,?,?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM clip_projects WHERE clip_projects.id = sqlc.arg(project_check) AND clip_projects.user_id = sqlc.arg(owner_check));

-- name: GetClipSpeechAsset :one
SELECT * FROM clip_speech_assets WHERE id = ? AND owner_id = ? AND project_id = ?;

-- name: FindClipSpeechAsset :one
SELECT * FROM clip_speech_assets WHERE owner_id = ? AND project_id = ? AND input_hash = ? AND binding_digest = ? ORDER BY created_at DESC LIMIT 1;

-- name: ListClipSpeechAssets :many
SELECT * FROM clip_speech_assets WHERE owner_id = ? AND project_id = ? ORDER BY created_at;
