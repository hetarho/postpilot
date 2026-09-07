package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// fakeStore is an in-memory auth.Store. It exists because these tests are about the
// service's rules (what a failure reveals, when a session dies), not about SQL.
type fakeStore struct {
	users    map[string]User
	sessions map[string]Session
	links    map[string]Link

	createSessionErr error
	deleteCalls      []string
	passwordOps      []string
	failureRecords   []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[string]User{}, sessions: map[string]Session{}, links: map[string]Link{}}
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

func (f *fakeStore) GetUserByEmail(_ context.Context, email string) (User, error) {
	for _, user := range f.users {
		if user.Email == email {
			return user, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (f *fakeStore) SetEmail(_ context.Context, id, email string, verifiedAt *time.Time) error {
	user, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.Email = email
	user.EmailVerifiedAt = verifiedAt
	f.users[id] = user
	return nil
}

func (f *fakeStore) MarkEmailVerified(_ context.Context, id string, at time.Time) error {
	user, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.EmailVerifiedAt = &at
	f.users[id] = user
	return nil
}

func (f *fakeStore) MarkEmailUnreachable(_ context.Context, id string, at time.Time) error {
	user, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.EmailUnreachableAt = &at
	f.users[id] = user
	return nil
}

func (f *fakeStore) UpdatePasswordHash(_ context.Context, id, passwordHash string) error {
	user, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	f.passwordOps = append(f.passwordOps, "update")
	user.PasswordHash = passwordHash
	f.users[id] = user
	return nil
}

func (f *fakeStore) RecordLoginFailure(_ context.Context, id string, now time.Time) (int, error) {
	f.failureRecords = append(f.failureRecords, id)
	user, ok := f.users[id]
	if !ok {
		return 0, ErrUserNotFound
	}
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		return 0, nil
	}
	count := user.FailedLogins + 1
	if count >= LockThreshold {
		user.FailedLogins = 0
		lockedUntil := now.Add(LockDuration)
		user.LockedUntil = &lockedUntil
	} else {
		user.FailedLogins = count
	}
	f.users[id] = user
	return count, nil
}

func (f *fakeStore) ClearLoginFailures(_ context.Context, id string) error {
	user, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.FailedLogins = 0
	user.LockedUntil = nil
	f.users[id] = user
	return nil
}

func (f *fakeStore) GetUserPlan(_ context.Context, id string) (plan.Plan, error) {
	u, ok := f.users[id]
	if !ok {
		return "", ErrUserNotFound
	}
	return u.Plan, nil
}

func (f *fakeStore) GetUserCreatedAt(_ context.Context, id string) (time.Time, error) {
	u, ok := f.users[id]
	if !ok {
		return time.Time{}, ErrUserNotFound
	}
	return u.CreatedAt, nil
}

// SetUserPlan mirrors the real statement's guard, which refuses to demote the only master.
func (f *fakeStore) SetUserPlan(_ context.Context, id string, p plan.Plan) error {
	u, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	if u.Plan == plan.Master && p != plan.Master && f.masters() <= 1 {
		return ErrLastMaster
	}
	u.Plan = p
	f.users[id] = u
	return nil
}

func (f *fakeStore) masters() int {
	var n int
	for _, u := range f.users {
		if u.Plan == plan.Master {
			n++
		}
	}
	return n
}

func (f *fakeStore) ListUsers(_ context.Context) ([]User, error) {
	out := make([]User, 0, len(f.users))
	for _, u := range f.users {
		out = append(out, u)
	}
	return out, nil
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

func (f *fakeStore) DeleteSessionsForUser(_ context.Context, userID string) error {
	f.passwordOps = append(f.passwordOps, "delete-sessions")
	for token, session := range f.sessions {
		if session.UserID == userID {
			delete(f.sessions, token)
		}
	}
	return nil
}

func (f *fakeStore) CreateLink(_ context.Context, link Link) error {
	f.links[link.TokenHash] = link
	return nil
}

func (f *fakeStore) ConsumeLink(_ context.Context, tokenHash string, purpose LinkPurpose, now time.Time) (Link, error) {
	link, ok := f.links[tokenHash]
	if !ok || link.Purpose != purpose || link.UsedAt != nil || !now.Before(link.ExpiresAt) {
		return Link{}, ErrLinkInvalid
	}
	link.UsedAt = &now
	f.links[tokenHash] = link
	return link, nil
}

func (f *fakeStore) InvalidateLinks(_ context.Context, userID string, purpose LinkPurpose, now time.Time) error {
	for token, link := range f.links {
		if link.UserID == userID && link.Purpose == purpose && link.UsedAt == nil {
			link.UsedAt = &now
			f.links[token] = link
		}
	}
	return nil
}

type fakeMailer struct {
	sent []Mail
	err  error
}

func (m *fakeMailer) Send(_ context.Context, mail Mail) error {
	m.sent = append(m.sent, mail)
	return m.err
}

func verificationToken(t *testing.T, mail Mail) string {
	t.Helper()
	for _, line := range strings.Split(mail.Text, "\n") {
		parsed, err := url.Parse(line)
		if err == nil && parsed.Path == "/verify-email" && parsed.Query().Get("token") != "" {
			return parsed.Query().Get("token")
		}
	}
	t.Fatalf("verification mail has no token URL: %s", mail.Text)
	return ""
}

func passwordResetToken(t *testing.T, mail Mail) string {
	t.Helper()
	for _, line := range strings.Split(mail.Text, "\n") {
		parsed, err := url.Parse(line)
		if err == nil && parsed.Path == "/reset-password" && parsed.Query().Get("token") != "" {
			return parsed.Query().Get("token")
		}
	}
	t.Fatalf("password reset mail has no token URL: %s", mail.Text)
	return ""
}

// newTestService returns a service with a frozen clock and a seeded account.
func newTestService(t *testing.T, now time.Time) (*Service, *fakeStore) {
	t.Helper()

	store := newFakeStore()
	svc := NewService(store, 720*time.Hour)
	svc.now = func() time.Time { return now }

	if err := svc.CreateUser(context.Background(), "alice", "s3cret", plan.Free); err != nil {
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

func TestLoginFailuresLockNotifyOnceAndReleaseAtTheBoundary(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	svc.now = func() time.Time { return now }
	verifiedAt := now.Add(-time.Hour)
	user := store.users["alice"]
	user.Email = "alice@example.com"
	user.EmailVerifiedAt = &verifiedAt
	store.users[user.ID] = user
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)

	for attempt := 1; attempt <= LockThreshold; attempt++ {
		if _, _, err := svc.Login(context.Background(), "alice", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("failure %d = %v", attempt, err)
		}
		stored := store.users["alice"]
		if attempt < LockThreshold {
			if stored.FailedLogins != attempt || stored.LockedUntil != nil || len(mailer.sent) != 0 {
				t.Fatalf("failure %d stored=%+v mails=%d", attempt, stored, len(mailer.sent))
			}
		}
	}
	locked := store.users["alice"]
	wantUntil := now.Add(LockDuration)
	if locked.FailedLogins != 0 || locked.LockedUntil == nil || !locked.LockedUntil.Equal(wantUntil) {
		t.Fatalf("locked user = %+v, want zero counter until %v", locked, wantUntil)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].To != "alice@example.com" || !strings.Contains(mailer.sent[0].Text, wantUntil.Format(time.RFC3339)) {
		t.Fatalf("lock mail = %+v", mailer.sent)
	}

	recordedAtLock := len(store.failureRecords)
	now = wantUntil.Add(-time.Second)
	realVerify := svc.verify
	var lockedVerifications []string
	svc.verify = func(password, encoded string) (bool, error) {
		lockedVerifications = append(lockedVerifications, encoded)
		return realVerify(password, encoded)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("correct password at 14m59s = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password while locked = %v, want ErrInvalidCredentials", err)
	}
	if len(store.failureRecords) != recordedAtLock || len(mailer.sent) != 1 {
		t.Fatalf("locked login recorded or mailed again: records=%d mails=%d", len(store.failureRecords), len(mailer.sent))
	}
	if len(lockedVerifications) != 2 || lockedVerifications[0] != locked.PasswordHash || lockedVerifications[1] != locked.PasswordHash {
		t.Fatalf("locked login verification hashes = %#v, want the real stored hash twice", lockedVerifications)
	}

	now = wantUntil
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatalf("correct password at 15m = %v", err)
	}
	released := store.users["alice"]
	if released.FailedLogins != 0 || released.LockedUntil != nil {
		t.Fatalf("released user retained lock state: %+v", released)
	}
}

func TestLoginSuccessClearsFailuresAndUnknownIDRecordsNothing(t *testing.T) {
	svc, store := newTestService(t, time.Now())
	if _, _, err := svc.Login(context.Background(), "alice", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	if store.users["alice"].FailedLogins != 1 {
		t.Fatalf("failed logins = %d, want 1", store.users["alice"].FailedLogins)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if store.users["alice"].FailedLogins != 0 {
		t.Fatalf("successful login left %d failures", store.users["alice"].FailedLogins)
	}

	before := len(store.failureRecords)
	if _, _, err := svc.Login(context.Background(), "nobody", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	if len(store.failureRecords) != before {
		t.Fatal("unknown id touched the account failure counter")
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

func TestSendSkipsAndRecordsUnreachableAddresses(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)
	user := store.users["alice"]
	user.Email = "alice@example.com"
	store.users[user.ID] = user
	message := existingAccountMail(user.Email)

	unreachableAt := now.Add(-time.Hour)
	user.EmailUnreachableAt = &unreachableAt
	if err := svc.send(context.Background(), user, message); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 0 {
		t.Fatal("mail was sent to an address already marked unreachable")
	}

	user.EmailUnreachableAt = nil
	mailer.err = ErrRecipientRejected
	if err := svc.send(context.Background(), user, message); err != nil {
		t.Fatal(err)
	}
	marked := store.users[user.ID].EmailUnreachableAt
	if marked == nil || !marked.Equal(now) {
		t.Fatalf("rejected recipient marked at %v, want %v", marked, now)
	}
}

func TestSendFailsWhenNoMailerIsWired(t *testing.T) {
	svc, store := newTestService(t, time.Now())
	user := store.users["alice"]
	user.Email = "alice@example.com"
	if err := svc.send(context.Background(), user, existingAccountMail(user.Email)); err == nil {
		t.Fatal("send silently dropped mail with no adapter")
	}
}

func TestSignupCreatesUnverifiedFreeAccountAndTakenAddressLooksSuccessful(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store := newFakeStore()
	svc := NewService(store, time.Hour)
	svc.now = func() time.Time { return now }
	svc.SetWebOrigin("https://postpilot.example.com")
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)
	bootstraps := 0
	svc.SetBootstraps(func(_ context.Context, userID string) error {
		if userID != "alice@example.com" {
			t.Fatalf("bootstrap user = %q", userID)
		}
		bootstraps++
		return nil
	})

	if err := svc.Signup(context.Background(), " Alice@Example.COM ", "password1"); err != nil {
		t.Fatal(err)
	}
	created := store.users["alice@example.com"]
	if created.ID != "alice@example.com" || created.Email != "alice@example.com" ||
		created.EmailVerifiedAt != nil || created.Plan != plan.Free || created.PasswordHash == "" {
		t.Fatalf("created user = %+v", created)
	}
	if bootstraps != 1 || len(mailer.sent) != 1 || !strings.Contains(mailer.sent[0].Subject, "Verify") {
		t.Fatalf("bootstraps=%d mail=%+v", bootstraps, mailer.sent)
	}

	if err := svc.Signup(context.Background(), "ALICE@example.com", "different-password"); err != nil {
		t.Fatalf("taken signup = %v, want the same nil response", err)
	}
	if len(store.users) != 1 || bootstraps != 1 {
		t.Fatalf("taken signup wrote state: users=%d bootstraps=%d", len(store.users), bootstraps)
	}
	if len(mailer.sent) != 2 || !strings.Contains(mailer.sent[1].Subject, "Account notice") {
		t.Fatalf("taken signup mail = %+v", mailer.sent)
	}
}

func TestSignupValidatesPasswordLength(t *testing.T) {
	svc := NewService(newFakeStore(), time.Hour)
	for _, tc := range []struct {
		password string
		want     error
	}{
		{strings.Repeat("a", PasswordMinLen-1), ErrPasswordTooShort},
		{strings.Repeat("a", PasswordMaxLen+1), ErrPasswordTooLong},
	} {
		if err := svc.Signup(context.Background(), "alice@example.com", tc.password); !errors.Is(err, tc.want) {
			t.Errorf("password length %d: %v, want %v", len(tc.password), err, tc.want)
		}
	}
}

func TestVerifyEmailIsSingleUseExpiresAndRunsBootstrapsWithoutASession(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store := newFakeStore()
	svc := NewService(store, time.Hour)
	svc.now = func() time.Time { return now }
	svc.SetWebOrigin("https://postpilot.example.com")
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)
	bootstraps := 0
	svc.SetBootstraps(func(context.Context, string) error { bootstraps++; return nil })
	if err := svc.Signup(context.Background(), "alice@example.com", "password1"); err != nil {
		t.Fatal(err)
	}
	raw := verificationToken(t, mailer.sent[0])
	if err := svc.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	verified := store.users["alice@example.com"].EmailVerifiedAt
	if verified == nil || !verified.Equal(now) || bootstraps != 2 || len(store.sessions) != 0 {
		t.Fatalf("verified=%v bootstraps=%d sessions=%d", verified, bootstraps, len(store.sessions))
	}
	if err := svc.VerifyEmail(context.Background(), raw); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("replay = %v, want ErrLinkInvalid", err)
	}

	expiredRaw, expiredHash, err := NewLinkToken()
	if err != nil {
		t.Fatal(err)
	}
	store.links[expiredHash] = Link{
		TokenHash: expiredHash, UserID: "alice@example.com", Purpose: LinkPurposeVerifyEmail,
		Email: "alice@example.com", ExpiresAt: now, CreatedAt: now.Add(-VerifyLinkTTL),
	}
	if err := svc.VerifyEmail(context.Background(), expiredRaw); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("expired = %v, want ErrLinkInvalid", err)
	}
}

func TestResendVerificationHidesAccountStateAndFloorsBursts(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	store := newFakeStore()
	svc := NewService(store, time.Hour)
	svc.now = func() time.Time { return now }
	svc.SetWebOrigin("https://postpilot.example.com")
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)

	if err := svc.ResendVerification(context.Background(), "unknown@example.com"); err != nil {
		t.Fatalf("unknown = %v", err)
	}
	verifiedAt := now.Add(-time.Hour)
	store.users["verified@example.com"] = User{
		ID: "verified@example.com", Email: "verified@example.com", EmailVerifiedAt: &verifiedAt,
	}
	if err := svc.ResendVerification(context.Background(), "verified@example.com"); err != nil {
		t.Fatalf("verified = %v", err)
	}
	store.users["waiting@example.com"] = User{ID: "waiting@example.com", Email: "waiting@example.com"}
	for range 3 {
		if err := svc.ResendVerification(context.Background(), "WAITING@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("burst sent %d messages, want one", len(mailer.sent))
	}
	now = now.Add(ResendFloor)
	if err := svc.ResendVerification(context.Background(), "waiting@example.com"); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 2 {
		t.Fatalf("post-floor messages = %d, want two", len(mailer.sent))
	}
}

func TestLoginByEmailAndUnverifiedOrGoogleOnlyCredentialsStayGeneric(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	svc.SetWebOrigin("https://postpilot.example.com")
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)

	verifiedAt := now
	legacy := store.users["alice"]
	legacy.Email = "alice@example.com"
	legacy.EmailVerifiedAt = &verifiedAt
	store.users["alice"] = legacy
	if user, _, err := svc.Login(context.Background(), "ALICE@EXAMPLE.COM", "s3cret"); err != nil || user.ID != "alice" {
		t.Fatalf("email login = %+v, %v", user, err)
	}

	waiting := legacy
	waiting.ID = "waiting@example.com"
	waiting.Email = waiting.ID
	waiting.EmailVerifiedAt = nil
	store.users[waiting.ID] = waiting
	_, _, wrong := svc.Login(context.Background(), waiting.ID, "wrong")
	_, _, correct := svc.Login(context.Background(), waiting.ID, "s3cret")
	if !errors.Is(wrong, ErrInvalidCredentials) || wrong.Error() != correct.Error() || len(mailer.sent) != 1 {
		t.Fatalf("wrong=%v correct=%v mail=%d", wrong, correct, len(mailer.sent))
	}

	store.users["google@example.com"] = User{
		ID: "google@example.com", Email: "google@example.com", EmailVerifiedAt: &verifiedAt,
	}
	var verifiedHash string
	svc.verify = func(_ string, encoded string) (bool, error) { verifiedHash = encoded; return false, nil }
	if _, _, err := svc.Login(context.Background(), "google@example.com", "guess"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	if verifiedHash != dummyHash() {
		t.Errorf("Google-only account verified against %q, want dummy hash", verifiedHash)
	}
}

func TestRegisterEmailHidesTakenAddressThenVerifiesTheHappyPath(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	svc.SetWebOrigin("https://postpilot.example.com")
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)
	store.users["bob"] = User{ID: "bob", Email: "taken@example.com", Plan: plan.Free}

	if err := svc.RegisterEmail(context.Background(), "alice", "taken@example.com"); err != nil {
		t.Fatalf("taken = %v, want nil", err)
	}
	if store.users["alice"].Email != "" || len(mailer.sent) != 1 || mailer.sent[0].To != "taken@example.com" {
		t.Fatalf("alice=%+v mail=%+v", store.users["alice"], mailer.sent)
	}

	if err := svc.RegisterEmail(context.Background(), "alice", "alice@example.com"); err != nil {
		t.Fatal(err)
	}
	raw := verificationToken(t, mailer.sent[1])
	if err := svc.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if store.users["alice"].EmailVerifiedAt == nil {
		t.Fatal("registered email was not verified")
	}
	if err := svc.RegisterEmail(context.Background(), "alice", "other@example.com"); !errors.Is(err, ErrEmailAlreadyVerified) {
		t.Fatalf("verified account register = %v, want ErrEmailAlreadyVerified", err)
	}
}

func TestRequestPasswordResetHidesAccountStateInvalidatesOlderLinksAndFloorsBursts(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	svc.now = func() time.Time { return now }
	svc.SetWebOrigin("https://postpilot.example.com")
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)

	verifiedAt := now.Add(-time.Hour)
	verified := store.users["alice"]
	verified.Email = "alice@example.com"
	verified.EmailVerifiedAt = &verifiedAt
	store.users[verified.ID] = verified
	store.users["waiting@example.com"] = User{
		ID: "waiting@example.com", Email: "waiting@example.com", Plan: plan.Free,
	}

	for _, email := range []string{"missing@example.com", "waiting@example.com", "ALICE@EXAMPLE.COM"} {
		if err := svc.RequestPasswordReset(context.Background(), email); err != nil {
			t.Errorf("RequestPasswordReset(%q) = %v, want nil", email, err)
		}
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("reset requests sent %d messages, want only the verified account's one", len(mailer.sent))
	}
	first := passwordResetToken(t, mailer.sent[0])
	for range 3 {
		if err := svc.RequestPasswordReset(context.Background(), "alice@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("reset burst sent %d messages, want one", len(mailer.sent))
	}

	now = now.Add(ResendFloor)
	if err := svc.RequestPasswordReset(context.Background(), "alice@example.com"); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 2 {
		t.Fatalf("post-floor reset messages = %d, want two", len(mailer.sent))
	}
	if err := svc.ResetPassword(context.Background(), first, "new-password"); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("older reset link = %v, want ErrLinkInvalid", err)
	}
	secondHash := hashToken(passwordResetToken(t, mailer.sent[1]))
	if got := store.links[secondHash].ExpiresAt; !got.Equal(now.Add(ResetLinkTTL)) {
		t.Fatalf("reset expiry = %v, want %v", got, now.Add(ResetLinkTTL))
	}
}

func TestResetPasswordIsSingleUseExpiresRevokesSessionsAndPreservesLock(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	svc.SetWebOrigin("https://postpilot.example.com")
	mailer := &fakeMailer{}
	svc.SetMailer(mailer)
	verifiedAt := now.Add(-time.Hour)
	lockedUntil := now.Add(15 * time.Minute)
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatal(err)
	}
	user := store.users["alice"]
	user.Email = "alice@example.com"
	user.EmailVerifiedAt = &verifiedAt
	user.FailedLogins = 4
	user.LockedUntil = &lockedUntil
	store.users[user.ID] = user
	if err := svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatal(err)
	}
	raw := passwordResetToken(t, mailer.sent[0])
	store.passwordOps = nil
	if err := svc.ResetPassword(context.Background(), raw, "new-password"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(store.passwordOps, ","); got != "update,delete-sessions" {
		t.Fatalf("password operations = %q, want update,delete-sessions", got)
	}
	if len(store.sessions) != 0 {
		t.Fatalf("sessions after reset = %d, want 0", len(store.sessions))
	}
	changed := store.users["alice"]
	if changed.FailedLogins != 4 || changed.LockedUntil == nil || !changed.LockedUntil.Equal(lockedUntil) {
		t.Fatalf("reset changed lock state: %+v", changed)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password login = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "new-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("new password bypassed the preserved lock: %v", err)
	}
	svc.now = func() time.Time { return lockedUntil }
	if _, _, err := svc.Login(context.Background(), "alice", "new-password"); err != nil {
		t.Fatalf("new password after lock release: %v", err)
	}
	if err := svc.ResetPassword(context.Background(), raw, "another-password"); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("reset replay = %v, want ErrLinkInvalid", err)
	}

	expiredRaw, expiredHash, err := NewLinkToken()
	if err != nil {
		t.Fatal(err)
	}
	store.links[expiredHash] = Link{
		TokenHash: expiredHash, UserID: "alice", Purpose: LinkPurposeResetPassword,
		Email: user.Email, ExpiresAt: now, CreatedAt: now.Add(-ResetLinkTTL),
	}
	if err := svc.ResetPassword(context.Background(), expiredRaw, "another-password"); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("expired reset = %v, want ErrLinkInvalid", err)
	}
}

func TestChangePasswordChecksCurrentPasswordAndRevokesEverySession(t *testing.T) {
	svc, store := newTestService(t, time.Now())
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(context.Background(), "alice", "wrong", "new-password"); !errors.Is(err, ErrCurrentPasswordWrong) {
		t.Fatalf("wrong current password = %v, want ErrCurrentPasswordWrong", err)
	}
	if len(store.sessions) != 2 {
		t.Fatalf("wrong current password deleted sessions: %d remain", len(store.sessions))
	}

	store.passwordOps = nil
	if err := svc.ChangePassword(context.Background(), "alice", "s3cret", "new-password"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(store.passwordOps, ","); got != "update,delete-sessions" {
		t.Fatalf("password operations = %q, want update,delete-sessions", got)
	}
	if len(store.sessions) != 0 {
		t.Fatalf("sessions after change = %d, want 0", len(store.sessions))
	}
	if _, _, err := svc.Login(context.Background(), "alice", "new-password"); err != nil {
		t.Fatalf("new password login: %v", err)
	}

	google := store.users["alice"]
	google.PasswordHash = ""
	store.users[google.ID] = google
	if err := svc.ChangePassword(context.Background(), "alice", "anything", "another-password"); !errors.Is(err, ErrPasswordNotSet) {
		t.Fatalf("Google-only change = %v, want ErrPasswordNotSet", err)
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
		actor, err := svc.Authenticate(context.Background(), raw)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if actor.UserID != "alice" {
			t.Errorf("user id = %q, want alice", actor.UserID)
		}
		if actor.Plan != plan.Free {
			t.Errorf("plan = %q, want free", actor.Plan)
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

	if err := svc.CreateUser(context.Background(), "alice", "another", plan.Free); !errors.Is(err, ErrDuplicateUser) {
		t.Errorf("duplicate id: %v, want ErrDuplicateUser", err)
	}
	if err := svc.CreateUser(context.Background(), "  ", "pw", plan.Free); err == nil {
		t.Error("a blank login id was accepted")
	}
	if err := svc.CreateUser(context.Background(), "bob", "", plan.Free); err == nil {
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
