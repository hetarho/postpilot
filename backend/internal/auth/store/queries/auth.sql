-- Queries for the auth context. sqlc compiles these into internal/auth/store/sqlc;
-- internal/auth/store maps the generated rows to domain types.

-- name: CreateUser :exec
INSERT INTO users (id, password_hash, plan, created_at) VALUES (?, ?, ?, ?);

-- name: GetUser :one
SELECT id, password_hash, plan, created_at FROM users WHERE id = ?;

-- name: GetUserPlan :one
SELECT plan FROM users WHERE id = ?;

-- name: SetUserPlan :execrows
-- The last-master guard is part of the statement, not a check before it: two concurrent
-- demotions that each counted two masters would both commit and leave the deployment with
-- none, and nothing could promote anyone back.
UPDATE users SET plan = ?
WHERE users.id = ?
  AND (users.plan <> 'master' OR (SELECT COUNT(*) FROM users AS m WHERE m.plan = 'master') > 1);

-- name: ListUsers :many
SELECT id, plan, created_at FROM users ORDER BY created_at, id;

-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?);

-- name: GetSessionByToken :one
SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < ?;
