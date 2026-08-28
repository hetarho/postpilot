package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// SessionCookieName is the wire name of the session cookie. The interceptor reads it
// and the rpc handler sets it; nothing else in the process needs to know it.
const SessionCookieName = "pp_session"

// Service is the auth context's behavior. It owns every rule about how a login
// succeeds, how long a session lives, and what a failure is allowed to reveal.
type Service struct {
	store Store
	ttl   time.Duration

	// now and verify are seams for tests in this package, not configuration. Keeping
	// them unexported means the production API has no test-only surface, while a
	// same-package test can freeze the clock or spy on the password verification that
	// the dummy path is required to perform.
	now    func() time.Time
	verify func(password, encoded string) (bool, error)
}

// NewService wires the context with its store and the session lifetime from config.
func NewService(store Store, ttl time.Duration) *Service {
	// Derive the dummy hash now, at boot, rather than on the first unknown-id login.
	// Deferred, that login would pay two argon2id derivations (build the dummy, then
	// verify against it) where a wrong password pays one — a timing difference on the
	// first probe after every restart, in exactly the direction the dummy exists to
	// erase.
	dummyHash()

	return &Service{store: store, ttl: ttl, now: time.Now, verify: VerifyPassword}
}

// Login verifies credentials and issues a session, returning the user and the RAW
// token for the cookie. The raw token is returned exactly once, here; it is never
// stored, logged, or placed in a response body.
//
// Every failure — unknown id, wrong password, unreadable stored hash — returns the
// same ErrInvalidCredentials after the same amount of argon2id work, so neither the
// message nor the timing tells a caller whether an id exists.
func (s *Service) Login(ctx context.Context, loginID, password string) (User, string, error) {
	user, err := s.store.GetUser(ctx, loginID)
	switch {
	case errors.Is(err, ErrUserNotFound):
		// The equalizing derivation. Its result is meaningless — running it is the point.
		_, _ = s.verify(password, dummyHash())
		return User{}, "", ErrInvalidCredentials
	case err != nil:
		return User{}, "", fmt.Errorf("load user: %w", err)
	}

	ok, err := s.verify(password, user.PasswordHash)
	if err != nil {
		// A stored hash that will not parse is an operator problem (a hand-edited row,
		// a botched restore). Record it, but answer the client exactly as for a wrong
		// password — the client learns nothing either way.
		slog.Error("stored password hash is unusable", "user_id", user.ID, "err", err)
		return User{}, "", ErrInvalidCredentials
	}
	if !ok {
		return User{}, "", ErrInvalidCredentials
	}

	raw, hashed, err := newSessionToken()
	if err != nil {
		return User{}, "", err
	}

	now := s.now()
	session := Session{
		Token:     hashed,
		UserID:    user.ID,
		ExpiresAt: now.Add(s.ttl),
		CreatedAt: now,
	}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return User{}, "", fmt.Errorf("create session: %w", err)
	}

	return user, raw, nil
}

// Authenticate resolves a raw cookie value to the acting user id.
//
// Absent, unknown, and expired all collapse into ErrNoSession: the caller gets 401
// either way, and distinguishing them would tell an attacker which of their guesses
// was once a real token.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (string, error) {
	if rawToken == "" {
		return "", ErrNoSession
	}

	session, err := s.store.GetSession(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, ErrNoSession) {
			return "", ErrNoSession
		}
		return "", fmt.Errorf("load session: %w", err)
	}

	if session.Expired(s.now()) {
		// The row just proved itself dead, so drop it here rather than waiting for the
		// next boot sweep. A failure to delete is not the caller's problem — the
		// expiry check above already denied them.
		if err := s.store.DeleteSession(ctx, session.Token); err != nil {
			slog.Warn("could not delete expired session", "err", err)
		}
		return "", ErrNoSession
	}

	return session.UserID, nil
}

// Logout revokes the session server-side. Clearing the cookie alone would leave a
// stolen copy valid for the rest of its 30 days.
//
// An unknown or absent token is not an error: logging out of a session that is already
// gone is the state the caller asked for.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	if err := s.store.DeleteSession(ctx, hashToken(rawToken)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// SweepExpired deletes sessions that are already past their expiry. Called once at
// boot; the per-request path also drops expired rows as it finds them, so this only
// collects sessions nobody ever came back for.
func (s *Service) SweepExpired(ctx context.Context) (int64, error) {
	n, err := s.store.DeleteExpiredSessions(ctx, s.now())
	if err != nil {
		return 0, fmt.Errorf("sweep expired sessions: %w", err)
	}
	return n, nil
}

// CreateUser provisions an account. It is reachable only from the operator command
// (auth/provision) — no RPC calls it, because postpilot has no signup (PRD F-1).
func (s *Service) CreateUser(ctx context.Context, loginID, password string) error {
	loginID = strings.TrimSpace(loginID)
	if loginID == "" {
		return errors.New("login id is required")
	}
	if password == "" {
		return errors.New("password is required")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	return s.store.CreateUser(ctx, User{
		ID:           loginID,
		PasswordHash: hash,
		CreatedAt:    s.now(),
	})
}
