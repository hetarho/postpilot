package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/plan"
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

// Authenticate resolves a raw cookie value to the acting caller.
//
// Absent, unknown, and expired all collapse into ErrNoSession: the caller gets 401
// either way, and distinguishing them would tell an attacker which of their guesses
// was once a real token.
//
// The plan is read here, on the session path, rather than by each gate: authority must
// be resolved once per request from the database, so a demotion takes effect on the
// caller's very next call instead of whenever their session happens to expire.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (Actor, error) {
	if rawToken == "" {
		return Actor{}, ErrNoSession
	}

	session, err := s.store.GetSession(ctx, hashToken(rawToken))
	if err != nil {
		if errors.Is(err, ErrNoSession) {
			return Actor{}, ErrNoSession
		}
		return Actor{}, fmt.Errorf("load session: %w", err)
	}

	if session.Expired(s.now()) {
		// The row just proved itself dead, so drop it here rather than waiting for the
		// next boot sweep. A failure to delete is not the caller's problem — the
		// expiry check above already denied them.
		if err := s.store.DeleteSession(ctx, session.Token); err != nil {
			slog.Warn("could not delete expired session", "err", err)
		}
		return Actor{}, ErrNoSession
	}

	acting, err := s.store.GetUserPlan(ctx, session.UserID)
	if err != nil {
		// A live session whose account is gone is a deleted user, not an outage: the
		// cascade removed the row and this token is meaningless.
		if errors.Is(err, ErrUserNotFound) {
			return Actor{}, ErrNoSession
		}
		return Actor{}, fmt.Errorf("load acting plan: %w", err)
	}

	return Actor{UserID: session.UserID, Plan: acting}, nil
}

// PlanOf reports an account's stored tier.
//
// It exists for the paths that act on behalf of a user without a live session — a worker
// running a queued job carries the process context, not the request's — where the row is
// the only authority available.
func (s *Service) PlanOf(ctx context.Context, userID string) (plan.Plan, error) {
	return s.store.GetUserPlan(ctx, userID)
}

// ListUsers returns every account for the operator screen, without password hashes.
func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

// SetUserPlan moves an account to another tier.
//
// Demoting the last master is refused rather than merely discouraged: master is the only
// tier that can promote anyone, so the deployment that loses its last one can never get
// another without shell access to the database. The refusal is enforced by the store's own
// statement, not by a count taken beforehand — two concurrent demotions would each see two
// masters and both commit.
func (s *Service) SetUserPlan(ctx context.Context, userID string, target plan.Plan) error {
	if !target.Valid() {
		return fmt.Errorf("unknown plan %q", target)
	}
	// A no-op set is not a demotion, and the guarded statement cannot tell the two apart:
	// setting the last master back to master would match zero rows and read as a refusal.
	current, err := s.store.GetUserPlan(ctx, userID)
	if err != nil {
		return err
	}
	if current == target {
		return nil
	}
	return s.store.SetUserPlan(ctx, userID, target)
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

// CreateUser provisions an account on a tier. It is reachable only from the operator
// command (auth/provision) — no RPC calls it, because postpilot has no signup (PRD F-1),
// which is also why the tier is an operator argument and never a request field.
func (s *Service) CreateUser(ctx context.Context, loginID, password string, tier plan.Plan) error {
	loginID = strings.TrimSpace(loginID)
	if loginID == "" {
		return errors.New("login id is required")
	}
	if password == "" {
		return errors.New("password is required")
	}
	if !tier.Valid() {
		return fmt.Errorf("unknown plan %q", tier)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	return s.store.CreateUser(ctx, User{
		ID:           loginID,
		PasswordHash: hash,
		Plan:         tier,
		CreatedAt:    s.now(),
	})
}
