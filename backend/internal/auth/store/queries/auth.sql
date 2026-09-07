-- Queries for the auth context. sqlc compiles these into internal/auth/store/sqlc;
-- internal/auth/store maps the generated rows to domain types.

-- name: CreateUser :exec
INSERT INTO users (
  id, password_hash, email, email_verified_at, email_unreachable_at,
  failed_logins, locked_until, google_subject, plan, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUser :one
SELECT id, password_hash, email, email_verified_at, email_unreachable_at,
       failed_logins, locked_until, google_subject, plan, created_at
FROM users WHERE id = ?;

-- name: GetUserByEmail :one
SELECT id, password_hash, email, email_verified_at, email_unreachable_at,
       failed_logins, locked_until, google_subject, plan, created_at
FROM users WHERE email = ?;

-- name: GetUserByGoogleSubject :one
SELECT id, password_hash, email, email_verified_at, email_unreachable_at,
       failed_logins, locked_until, google_subject, plan, created_at
FROM users WHERE google_subject = ?;

-- name: SetEmail :exec
UPDATE users
SET email = NULLIF(sqlc.arg(email), ''),
    email_verified_at = sqlc.narg(email_verified_at)
WHERE id = sqlc.arg(id);

-- name: BindGoogleIdentity :execrows
-- The subject check and verification side effect are one write. The guard prevents a
-- stale service read from replacing an identity another request has just attached.
UPDATE users
SET google_subject = sqlc.arg(google_subject),
    email_verified_at = COALESCE(email_verified_at, sqlc.arg(email_verified_at))
WHERE id = sqlc.arg(id)
  AND (google_subject IS NULL OR google_subject = sqlc.arg(google_subject));

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = ? WHERE id = ?;

-- name: MarkEmailUnreachable :exec
UPDATE users SET email_unreachable_at = ? WHERE id = ?;

-- name: UpdatePasswordHash :exec
-- Deliberately changes no lockout columns: a mailed credential must not unlock an account.
UPDATE users SET password_hash = ? WHERE id = ?;

-- name: RecordLoginFailure :one
-- The threshold transition is one write: the fifth failure returns five to the caller,
-- stores a zero counter for the next window, and installs the self-expiring lock.
UPDATE users
SET failed_logins = CASE
      WHEN failed_logins + 1 >= sqlc.arg(lock_threshold) THEN 0
      ELSE failed_logins + 1
    END,
    locked_until = CASE
      WHEN failed_logins + 1 >= sqlc.arg(lock_threshold) THEN sqlc.arg(new_locked_until)
      ELSE locked_until
    END
WHERE id = sqlc.arg(id)
  AND (locked_until IS NULL OR locked_until <= sqlc.arg(now))
RETURNING CAST(CASE
  WHEN failed_logins = 0 AND locked_until = sqlc.arg(new_locked_until) THEN sqlc.arg(lock_threshold)
  ELSE failed_logins
END AS INTEGER) AS failure_count;

-- name: ClearLoginFailures :exec
UPDATE users SET failed_logins = 0, locked_until = NULL WHERE id = ?;

-- name: GetUserPlan :one
SELECT plan FROM users WHERE id = ?;

-- name: GetUserCreatedAt :one
-- The usage anchor needs only this instant; never load the password hash for it.
SELECT created_at FROM users WHERE id = ?;

-- name: SetUserPlan :execrows
-- The last-master guard is part of the statement, not a check before it: two concurrent
-- demotions that each counted two masters would both commit and leave the deployment with
-- none, and nothing could promote anyone back.
UPDATE users SET plan = ?
WHERE users.id = ?
  AND (users.plan <> 'master' OR (SELECT COUNT(*) FROM users AS m WHERE m.plan = 'master') > 1);

-- name: ListUsers :many
SELECT id, email, email_verified_at, email_unreachable_at, failed_logins,
       locked_until, google_subject, plan, created_at
FROM users ORDER BY created_at, id;

-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, ?, ?);

-- name: GetSessionByToken :one
SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ?;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < ?;

-- name: DeleteSessionsForUser :exec
DELETE FROM sessions WHERE user_id = ?;

-- name: CreateLink :exec
INSERT INTO auth_links (
  token_hash, user_id, purpose, email, expires_at, used_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ConsumeLink :one
UPDATE auth_links SET used_at = ?
WHERE token_hash = ?
  AND purpose = ?
  AND used_at IS NULL
  AND expires_at > ?
RETURNING token_hash, user_id, purpose, email, expires_at, used_at, created_at;

-- name: InvalidateLinks :exec
UPDATE auth_links SET used_at = ?
WHERE user_id = ? AND purpose = ? AND used_at IS NULL;
