package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()

	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })

	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(handle.Writer, handle.Reader)
}

func TestUserRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	// A sub-second, non-UTC timestamp: the store must normalize it, and the value that
	// comes back must be the same instant.
	created := time.Date(2026, 3, 1, 12, 30, 45, 123456789, time.FixedZone("KST", 9*3600))
	want := auth.User{ID: "alice", PasswordHash: "$argon2id$...", Plan: plan.Basic, CreatedAt: created}

	if err := s.CreateUser(ctx, want); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.ID != want.ID || got.PasswordHash != want.PasswordHash || got.Plan != want.Plan {
		t.Errorf("user = %+v, want %+v", got, want)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("created_at = %v, want the same instant as %v", got.CreatedAt, want.CreatedAt)
	}
}

func TestGetUserUnknown(t *testing.T) {
	if _, err := newStore(t).GetUser(context.Background(), "nobody"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("error = %v, want ErrUserNotFound (sql.ErrNoRows must not escape the store)", err)
	}
}

// TestCreateUserDuplicate is plan 01 AC5's server half: the driver's UNIQUE violation
// must arrive as a domain error the operator command can explain.
func TestCreateUserDuplicate(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	user := auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}

	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.CreateUser(ctx, user); !errors.Is(err, auth.ErrDuplicateUser) {
		t.Errorf("error = %v, want ErrDuplicateUser", err)
	}
}

func TestSessionRoundTripAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	if err := s.CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	want := auth.Session{Token: "abc123", UserID: "alice", ExpiresAt: now.Add(720 * time.Hour), CreatedAt: now}
	if err := s.CreateSession(ctx, want); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.GetSession(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.UserID != "alice" || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("session = %+v, want %+v", got, want)
	}

	if err := s.DeleteSession(ctx, "abc123"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := s.GetSession(ctx, "abc123"); !errors.Is(err, auth.ErrNoSession) {
		t.Errorf("error after delete = %v, want ErrNoSession", err)
	}
}

// TestDeleteExpiredSessions also proves the RFC3339 storage format orders correctly as
// plain text — the whole reason expires_at is comparable in SQL without conversion.
func TestDeleteExpiredSessions(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	if err := s.CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: now}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	sessions := map[string]time.Time{
		"long-dead": now.Add(-30 * 24 * time.Hour),
		"just-dead": now.Add(-time.Second),
		"alive":     now.Add(time.Hour),
	}
	for token, expiry := range sessions {
		if err := s.CreateSession(ctx, auth.Session{Token: token, UserID: "alice", ExpiresAt: expiry, CreatedAt: now}); err != nil {
			t.Fatalf("CreateSession(%s): %v", token, err)
		}
	}

	n, err := s.DeleteExpiredSessions(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if n != 2 {
		t.Errorf("deleted %d sessions, want 2", n)
	}
	if _, err := s.GetSession(ctx, "alive"); err != nil {
		t.Errorf("the live session was swept: %v", err)
	}
}

// TestSessionCascadesWithUser proves foreign_keys=ON actually reached the writer —
// without it the ON DELETE CASCADE in the schema is silently inert.
func TestSessionCascadesWithUser(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	now := time.Now()

	if err := s.CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: now}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s.CreateSession(ctx, auth.Session{Token: "t", UserID: "alice", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// A session for an account that does not exist must be refused outright.
	err := s.CreateSession(ctx, auth.Session{Token: "orphan", UserID: "ghost", ExpiresAt: now.Add(time.Hour), CreatedAt: now})
	if err == nil {
		t.Error("a session referencing an unknown user was accepted — foreign keys are off")
	}
}

// TestTimestampsSortLexicographically pins the property the queries depend on: SQLite
// compares these as plain strings, so byte order must match chronological order. A
// variable-width fraction (time.RFC3339Nano trims trailing zeros) breaks it.
func TestTimestampsSortLexicographically(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	if err := s.CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: base}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// .5s formats as ".5" under RFC3339Nano and sorts AFTER ".513110616" byte-for-byte,
	// so under the buggy layout this session looks expired and gets swept.
	notYetExpired := base.Add(500 * time.Millisecond)
	if err := s.CreateSession(ctx, auth.Session{
		Token: "live", UserID: "alice", ExpiresAt: notYetExpired, CreatedAt: base,
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	n, err := s.DeleteExpiredSessions(ctx, base.Add(413110616*time.Nanosecond))
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if n != 0 {
		t.Errorf("swept %d sessions, want 0 — the session had not expired yet", n)
	}
	if _, err := s.GetSession(ctx, "live"); err != nil {
		t.Errorf("a live session was swept: %v", err)
	}
}
