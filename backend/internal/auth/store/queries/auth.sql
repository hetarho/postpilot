-- Queries for the auth context. sqlc compiles these into internal/auth/store/sqlc;
-- internal/auth/store maps the generated rows to domain types.

-- name: CreateUser :exec
INSERT INTO users (id, password_hash, created_at) VALUES (?, ?, ?);

-- name: GetUser :one
SELECT id, password_hash, created_at FROM users WHERE id = ?;

-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?);

-- name: GetSessionByToken :one
SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < ?;
