package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeStore is an in-memory auth.Store. It exists because these tests are about the
// service's rules (what a failure reveals, when a session dies), not about SQL.
type fakeStore struct {
	users    map[string]User
	sessions map[string]Session

	createSessionErr error
	deleteCalls      []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[string]User{}, sessions: map[string]Session{}}
}

func (f *fakeStore) CreateUser(_ context.Context, u User) error {
	if _, exists := f.users[u.ID]; exists {
		return ErrDuplicateUser
	}
	f.users[u.ID] = u
	return nil
}

func (f *fakeStore) GetUser(_ context.Context, id string) (User, error) {
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (f *fakeStore) CreateSession(_ context.Context, s Session) error {
	if f.createSessionErr != nil {
		return f.createSessionErr
	}
	f.sessions[s.Token] = s
	return nil
}

func (f *fakeStore) GetSession(_ context.Context, token string) (Session, error) {
	s, ok := f.sessions[token]
	if !ok {
		return Session{}, ErrNoSession
	}
	return s, nil
}

func (f *fakeStore) DeleteSession(_ context.Context, token string) error {
	f.deleteCalls = append(f.deleteCalls, token)
	delete(f.sessions, token)
	return nil
}

func (f *fakeStore) DeleteExpiredSessions(_ context.Context, before time.Time) (int64, error) {
	var n int64
	for token, s := range f.sessions {
		if s.Expired(before) {
			delete(f.sessions, token)
			n++
		}
	}
	return n, nil
}

// newTestService returns a service with a frozen clock and a seeded account.
func newTestService(t *testing.T, now time.Time) (*Service, *fakeStore) {
	t.Helper()

	store := newFakeStore()
	svc := NewService(store, 720*time.Hour)
	svc.now = func() time.Time { return now }

	if err := svc.CreateUser(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return svc, store
}

func TestLoginSuccessIssuesSession(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)

	user, raw, err := svc.Login(context.Background(), "alice", "s3cret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if user.ID != "alice" {
		t.Errorf("user id = %q, want alice", user.ID)
	}
	if raw == "" {
		t.Fatal("Login returned an empty token")
	}

	// The database must hold the hash, never the raw cookie value.
	if _, found := store.sessions[raw]; found {
		t.Fatal("the raw token was stored verbatim")
	}
	session, found := store.sessions[hashToken(raw)]
	if !found {
		t.Fatal("no session was stored under the hashed token")
	}
	if session.UserID != "alice" {
		t.Errorf("session user = %q, want alice", session.UserID)
	}
	if want := now.Add(720 * time.Hour); !session.ExpiresAt.Equal(want) {
		t.Errorf("expires_at = %v, want %v (login + 30d)", session.ExpiresAt, want)
	}
}

// TestLoginFailuresAreIdentical is plan 01 AC3: an unknown id and a wrong password
// must be indistinguishable in what they return AND in the work they do.
func TestLoginFailuresAreIdentical(t *testing.T) {
	svc, _ := newTestService(t, time.Now())

	// Spy on the verification so the dummy path is observable without timing anything.
	var verified []string
	realVerify := svc.verify
	svc.verify = func(password, encoded string) (bool, error) {
		verified = append(verified, encoded)
		return realVerify(password, encoded)
	}

	_, _, unknownErr := svc.Login(context.Background(), "nobody", "s3cret")
	unknownVerifications := len(verified)

	verified = nil
	_, _, wrongErr := svc.Login(context.Background(), "alice", "wrong")
	wrongVerifications := len(verified)

	if !errors.Is(unknownErr, ErrInvalidCredentials) {
		t.Errorf("unknown id error = %v, want ErrInvalidCredentials", unknownErr)
	}
	if !errors.Is(wrongErr, ErrInvalidCredentials) {
		t.Errorf("wrong password error = %v, want ErrInvalidCredentials", wrongErr)
	}
	if unknownErr.Error() != wrongErr.Error() {
		t.Errorf("error messages differ: %q vs %q", unknownErr, wrongErr)
	}

	if unknownVerifications != 1 {
		t.Errorf("unknown id ran %d argon2id verifications, want exactly 1", unknownVerifications)
	}
	if wrongVerifications != 1 {
		t.Errorf("wrong password ran %d argon2id verifications, want exactly 1", wrongVerifications)
	}
}

func TestLoginUnknownIDVerifiesAgainstTheDummyHash(t *testing.T) {
	svc, _ := newTestService(t, time.Now())

	var seen string
	svc.verify = func(_, encoded string) (bool, error) {
		seen = encoded
		return false, nil
	}

	if _, _, err := svc.Login(context.Background(), "nobody", "guess"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login: %v, want ErrInvalidCredentials", err)
	}
	if seen != dummyHash() {
		t.Errorf("unknown id verified against %q, want the dummy hash", seen)
	}
}

func TestLoginUnusableStoredHashLooksLikeAWrongPassword(t *testing.T) {
	svc, store := newTestService(t, time.Now())

	corrupted := store.users["alice"]
	corrupted.PasswordHash = "not-a-phc-string"
	store.users["alice"] = corrupted

	_, _, err := svc.Login(context.Background(), "alice", "s3cret")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("error = %v, want ErrInvalidCredentials (an operator problem must not leak)", err)
	}
}

// TestLoginStoreFailureIsNotACredentialError keeps an outage from masquerading as a
// rejected password: a store failure after successful verification must surface as
// itself, so the handler answers 500 rather than sending the operator hunting for a
// typo in their password.
func TestLoginStoreFailureIsNotACredentialError(t *testing.T) {
	svc, store := newTestService(t, time.Now())
	store.createSessionErr = errors.New("disk full")

	_, _, err := svc.Login(context.Background(), "alice", "s3cret")
	if err == nil {
		t.Fatal("Login succeeded despite a store failure")
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("error = %v, want an infrastructure error, not ErrInvalidCredentials", err)
	}
}

func TestAuthenticate(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)

	_, raw, err := svc.Login(context.Background(), "alice", "s3cret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	t.Run("valid", func(t *testing.T) {
		id, err := svc.Authenticate(context.Background(), raw)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if id != "alice" {
			t.Errorf("user id = %q, want alice", id)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := svc.Authenticate(context.Background(), ""); !errors.Is(err, ErrNoSession) {
			t.Errorf("error = %v, want ErrNoSession", err)
		}
	})

	t.Run("tampered by one character", func(t *testing.T) {
		// Plan 01 AC2 — the stored value is a hash, so a single flipped byte misses.
		tampered := flipFirstChar(raw)
		if tampered == raw {
			t.Fatal("could not tamper with the token")
		}
		if _, err := svc.Authenticate(context.Background(), tampered); !errors.Is(err, ErrNoSession) {
			t.Errorf("error = %v, want ErrNoSession", err)
		}
	})

	t.Run("expired is denied and swept", func(t *testing.T) {
		svc.now = func() time.Time { return now.Add(721 * time.Hour) }
		defer func() { svc.now = func() time.Time { return now } }()

		if _, err := svc.Authenticate(context.Background(), raw); !errors.Is(err, ErrNoSession) {
			t.Errorf("error = %v, want ErrNoSession", err)
		}
		if _, still := store.sessions[hashToken(raw)]; still {
			t.Error("an expired session survived the lookup that rejected it")
		}
	})
}

// TestSessionExpiryBoundary pins the exact instant a session dies: valid up to and
// including the last nanosecond before expires_at, dead at expires_at itself.
func TestSessionExpiryBoundary(t *testing.T) {
	expiry := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	session := Session{ExpiresAt: expiry}

	if session.Expired(expiry.Add(-time.Nanosecond)) {
		t.Error("session expired one nanosecond early")
	}
	if !session.Expired(expiry) {
		t.Error("session still valid at its own expires_at")
	}
	if !session.Expired(expiry.Add(time.Nanosecond)) {
		t.Error("session still valid past expires_at")
	}
}

func TestLogoutRevokesServerSide(t *testing.T) {
	svc, store := newTestService(t, time.Now())

	_, raw, err := svc.Login(context.Background(), "alice", "s3cret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := svc.Logout(context.Background(), raw); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	// Plan 01 AC6 — replaying the cookie must fail, not merely be un-sent.
	if _, err := svc.Authenticate(context.Background(), raw); !errors.Is(err, ErrNoSession) {
		t.Errorf("replayed cookie after logout: %v, want ErrNoSession", err)
	}
	if _, still := store.sessions[hashToken(raw)]; still {
		t.Error("the session row survived logout")
	}
}

func TestLogoutWithoutTokenIsNotAnError(t *testing.T) {
	svc, store := newTestService(t, time.Now())

	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Errorf("Logout(\"\") = %v, want nil", err)
	}
	if len(store.deleteCalls) != 0 {
		t.Errorf("Logout(\"\") hit the store %d times, want 0", len(store.deleteCalls))
	}
}

func TestSweepExpired(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)

	_, live, err := svc.Login(context.Background(), "alice", "s3cret")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	store.sessions["dead"] = Session{Token: "dead", UserID: "alice", ExpiresAt: now.Add(-time.Hour)}

	n, err := svc.SweepExpired(context.Background())
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("swept %d sessions, want 1", n)
	}
	if _, ok := store.sessions[hashToken(live)]; !ok {
		t.Error("the sweep deleted a live session")
	}
}

func TestCreateUserRejectsDuplicateAndBlanks(t *testing.T) {
	svc, _ := newTestService(t, time.Now())

	if err := svc.CreateUser(context.Background(), "alice", "another"); !errors.Is(err, ErrDuplicateUser) {
		t.Errorf("duplicate id: %v, want ErrDuplicateUser", err)
	}
	if err := svc.CreateUser(context.Background(), "  ", "pw"); err == nil {
		t.Error("a blank login id was accepted")
	}
	if err := svc.CreateUser(context.Background(), "bob", ""); err == nil {
		t.Error("a blank password was accepted")
	}
}

func TestCreateUserStoresOnlyAHash(t *testing.T) {
	_, store := newTestService(t, time.Now())

	stored := store.users["alice"].PasswordHash
	if strings.Contains(stored, "s3cret") {
		t.Fatalf("the plaintext password reached the store: %q", stored)
	}
	if ok, err := VerifyPassword("s3cret", stored); err != nil || !ok {
		t.Errorf("stored hash does not verify the seeded password (ok=%v err=%v)", ok, err)
	}
}

func flipFirstChar(s string) string {
	if s == "" {
		return s
	}
	replacement := byte('A')
	if s[0] == replacement {
		replacement = 'B'
	}
	return string(replacement) + s[1:]
}
