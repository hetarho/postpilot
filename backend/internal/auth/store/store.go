// Package store persists the auth context. It is the anti-corruption boundary on the
// database side (ARCHITECTURE §2.2): sqlc row structs and driver errors stop here, and
// only auth domain types travel inward.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/auth/store/sqlc"
)

// writeLayout is how timestamps live in SQLite. SQLite has no date type; RFC3339 in
// UTC sorts lexicographically, so `expires_at < ?` is an ordinary indexed string
// comparison and needs no conversion in SQL.
//
// The fraction is written at FIXED width. time.RFC3339Nano trims trailing zeros, and
// variable width breaks the ordering the queries depend on: "…08.5Z" compares greater
// than "…08.513110616Z" byte-for-byte, so a sweep would judge the wrong row expired.
const writeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Store implements auth.Store over SQLite.
//
// It holds both handles because the split is not an implementation detail it can hide:
// writes must serialize through the single writer connection, reads should not
// (ARCHITECTURE §2.4).
type Store struct {
	write *sqlc.Queries
	read  *sqlc.Queries
}

// New builds the store over the process's writer and reader pools.
func New(writer, reader *sql.DB) *Store {
	return &Store{write: sqlc.New(writer), read: sqlc.New(reader)}
}

func (s *Store) CreateUser(ctx context.Context, u auth.User) error {
	err := s.write.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           u.ID,
		PasswordHash: u.PasswordHash,
		CreatedAt:    formatTime(u.CreatedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return auth.ErrDuplicateUser
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (s *Store) GetUser(ctx context.Context, id string) (auth.User, error) {
	row, err := s.read.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, auth.ErrUserNotFound
		}
		return auth.User{}, fmt.Errorf("select user: %w", err)
	}

	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return auth.User{}, fmt.Errorf("user %s: %w", row.ID, err)
	}
	return auth.User{ID: row.ID, PasswordHash: row.PasswordHash, CreatedAt: createdAt}, nil
}

func (s *Store) CreateSession(ctx context.Context, sess auth.Session) error {
	err := s.write.CreateSession(ctx, sqlc.CreateSessionParams{
		Token:     sess.Token,
		UserID:    sess.UserID,
		ExpiresAt: formatTime(sess.ExpiresAt),
		CreatedAt: formatTime(sess.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (s *Store) GetSession(ctx context.Context, hashedToken string) (auth.Session, error) {
	row, err := s.read.GetSessionByToken(ctx, hashedToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Session{}, auth.ErrNoSession
		}
		return auth.Session{}, fmt.Errorf("select session: %w", err)
	}

	expiresAt, err := parseTime(row.ExpiresAt)
	if err != nil {
		return auth.Session{}, fmt.Errorf("session expires_at: %w", err)
	}
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return auth.Session{}, fmt.Errorf("session created_at: %w", err)
	}

	return auth.Session{
		Token:     row.Token,
		UserID:    row.UserID,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}, nil
}

func (s *Store) DeleteSession(ctx context.Context, hashedToken string) error {
	if err := s.write.DeleteSession(ctx, hashedToken); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error) {
	n, err := s.write.DeleteExpiredSessions(ctx, formatTime(before))
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return n, nil
}

// formatTime normalizes to UTC before formatting so stored values sort against each
// other regardless of the offset the caller's clock happened to carry.
func formatTime(t time.Time) string { return t.UTC().Format(writeLayout) }

// parseTime reads with RFC3339Nano rather than writeLayout: it accepts any RFC3339
// fraction width, so a row written before the width was pinned still loads.
func parseTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", v, err)
	}
	return t, nil
}

// isUniqueViolation detects a primary-key collision.
//
// It matches on the message rather than a driver error code: modernc.org/sqlite
// returns its own *sqlite.Error type, and importing it here just to read a code would
// pull the driver into a package whose whole job is to keep drivers at the edge. The
// message text is part of SQLite's stable public behavior.
func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToUpper(err.Error()), "UNIQUE CONSTRAINT FAILED")
}
