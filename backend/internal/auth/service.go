package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/plan"
)

// SessionCookieName is the wire name of the session cookie. The interceptor reads it
// and the rpc handler sets it; nothing else in the process needs to know it.
const SessionCookieName = "pp_session"

const (
	PasswordMinLen = 8
	PasswordMaxLen = 128
	ResendFloor    = 60 * time.Second
	LockThreshold  = 5
	LockDuration   = 15 * time.Minute
)

// Service is the auth context's behavior. It owns every rule about how a login
// succeeds, how long a session lives, and what a failure is allowed to reveal.
type Service struct {
	store      Store
	ttl        time.Duration
	mailer     Mailer
	google     GoogleIdentity
	webOrigin  string
	bootstraps []AccountBootstrap

	resendMu   sync.Mutex
	lastResend map[string]time.Time

	// now and verify are seams for tests in this package, not configuration. Keeping
	// them unexported means the production API has no test-only surface, while a
	// same-package test can freeze the clock or spy on the password verification that
	// the dummy path is required to perform.
	now    func() time.Time
	verify func(password, encoded string) (bool, error)
	// topUp raises the current monthly grant when a tier moves up. Late-bound because the
	// ledger that implements it is built after this service (the composition root wires it
	// with SetMonthlyTopUp, the way the catalog gets its reasoning-spend reader).
	topUp MonthlyTopUp
}

// NewService wires the context with its store and the session lifetime from config.
func NewService(store Store, ttl time.Duration) *Service {
	// Derive the dummy hash now, at boot, rather than on the first unknown-id login.
	// Deferred, that login would pay two argon2id derivations (build the dummy, then
	// verify against it) where a wrong password pays one — a timing difference on the
	// first probe after every restart, in exactly the direction the dummy exists to
	// erase.
	dummyHash()

	return &Service{
		store: store, ttl: ttl, now: time.Now, verify: VerifyPassword,
		lastResend: make(map[string]time.Time),
	}
}

// SetMonthlyTopUp gives the service the credit side of a tier upgrade. Without it an
// upgrade that owes credits fails loudly rather than moving the tier and dropping the
// grant on the floor.
func (s *Service) SetMonthlyTopUp(topUp MonthlyTopUp) { s.topUp = topUp }

// SetBootstraps replaces the idempotent account defaults. Replacement lets a repaired
// dependency heal an account when its verification link is consumed later.
func (s *Service) SetBootstraps(bootstraps ...AccountBootstrap) {
	s.bootstraps = append([]AccountBootstrap(nil), bootstraps...)
}

// SetMailer attaches the delivery edge after the auth service is constructed. A caller that
// reaches send without this wiring receives an error; transactional mail is never dropped.
func (s *Service) SetMailer(mailer Mailer) { s.mailer = mailer }

// SetGoogle attaches the optional Google authorization-code exchange edge. Leaving it
// nil is the supported disabled configuration rather than an incomplete service.
func (s *Service) SetGoogle(identity GoogleIdentity) { s.google = identity }

// SetWebOrigin supplies the browser origin used to build verification and reset URLs.
func (s *Service) SetWebOrigin(origin string) {
	s.webOrigin = strings.TrimRight(strings.TrimSpace(origin), "/")
}

// send applies the permanent-recipient policy before and after the delivery adapter.
func (s *Service) send(ctx context.Context, user User, mail Mail) error {
	if user.EmailUnreachableAt != nil {
		slog.InfoContext(ctx, "transactional mail skipped for unreachable address", "user_id", user.ID, "to", mail.To)
		return nil
	}
	if s.mailer == nil {
		return errors.New("auth mailer is not wired")
	}
	if err := s.mailer.Send(ctx, mail); err != nil {
		if !errors.Is(err, ErrRecipientRejected) {
			return fmt.Errorf("send transactional mail: %w", err)
		}
		if err := s.store.MarkEmailUnreachable(ctx, user.ID, s.now()); err != nil {
			return fmt.Errorf("mark rejected email unreachable: %w", err)
		}
		return nil
	}
	return nil
}

// Signup creates an unverified free account, or mails the owner of an address that is
// already present. Both branches hash the submitted password before they decide so the
// public response does not become an address-existence timing oracle.
func (s *Service) Signup(ctx context.Context, rawEmail, password string) error {
	email, err := normalizedEmail(rawEmail)
	if err != nil {
		return err
	}
	if err := validatePasswordLength(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	existing, found, err := s.accountForEmail(ctx, email)
	if err != nil {
		return err
	}
	if found {
		return s.send(ctx, existing, existingAccountMail(email))
	}

	user := User{
		ID: email, PasswordHash: hash, Email: email, Plan: plan.Free, CreatedAt: s.now(),
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		if !errors.Is(err, ErrDuplicateUser) {
			return fmt.Errorf("create signup account: %w", err)
		}
		// A concurrent request won the insert. Keep the same public success shape and mail
		// the owner just like the ordinary taken-address branch.
		existing, found, loadErr := s.accountForEmail(ctx, email)
		if loadErr != nil {
			return fmt.Errorf("resolve concurrently created account: %w", loadErr)
		}
		if !found {
			return errors.New("resolve concurrently created account: duplicate account disappeared")
		}
		return s.send(ctx, existing, existingAccountMail(email))
	}
	if err := s.runBootstraps(ctx, user.ID); err != nil {
		return fmt.Errorf("bootstrap signup account: %w", err)
	}
	return s.sendVerification(ctx, user)
}

// ResendVerification deliberately reports no account state. Only an unverified address
// receives mail, and the per-process floor collapses a burst to one delivery.
func (s *Service) ResendVerification(ctx context.Context, rawEmail string) error {
	email, err := normalizedEmail(rawEmail)
	if err != nil {
		return err
	}
	user, found, err := s.accountForEmail(ctx, email)
	if err != nil || !found {
		return err
	}
	if user.EmailVerifiedAt != nil {
		return nil
	}
	user.Email = email
	return s.sendVerification(ctx, user)
}

// RequestPasswordReset builds the same credential material before it knows whether an
// address can receive it. Only a verified owner gets a stored link and mail; every valid
// address receives the same nil result.
func (s *Service) RequestPasswordReset(ctx context.Context, rawEmail string) error {
	email, err := normalizedEmail(rawEmail)
	if err != nil {
		return err
	}
	raw, tokenHash, err := NewLinkToken()
	if err != nil {
		return err
	}
	linkURL, err := s.passwordResetLink(raw)
	if err != nil {
		return err
	}
	mail := passwordResetMail(email, linkURL)

	user, found, err := s.accountForEmail(ctx, email)
	if err != nil || !found {
		return err
	}
	if user.EmailVerifiedAt == nil {
		return nil
	}
	now := s.now()
	if !s.reserveResend(email, now) {
		return nil
	}
	if err := s.store.InvalidateLinks(ctx, user.ID, LinkPurposeResetPassword, now); err != nil {
		s.releaseResend(email, now)
		return fmt.Errorf("invalidate password reset links: %w", err)
	}
	if err := s.store.CreateLink(ctx, Link{
		TokenHash: tokenHash, UserID: user.ID, Purpose: LinkPurposeResetPassword,
		Email: email, ExpiresAt: now.Add(ResetLinkTTL), CreatedAt: now,
	}); err != nil {
		s.releaseResend(email, now)
		return fmt.Errorf("create password reset link: %w", err)
	}
	if err := s.send(ctx, user, mail); err != nil {
		s.releaseResend(email, now)
		return err
	}
	return nil
}

// VerifyEmail consumes one verification credential, marks the address, and repairs every
// idempotent account default. It never creates a session.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrLinkInvalid
	}
	link, err := s.store.ConsumeLink(ctx, hashToken(rawToken), LinkPurposeVerifyEmail, s.now())
	if err != nil {
		if errors.Is(err, ErrLinkInvalid) {
			return ErrLinkInvalid
		}
		return fmt.Errorf("consume verification link: %w", err)
	}
	if err := s.store.MarkEmailVerified(ctx, link.UserID, s.now()); err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	if err := s.runBootstraps(ctx, link.UserID); err != nil {
		return fmt.Errorf("bootstrap verified account: %w", err)
	}
	return nil
}

// ResetPassword proves ownership with a single-use mailed credential, then replaces the
// password and revokes every session. Password validation and hashing happen first so a
// correct link is not burned by a fixable form error.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if err := validatePasswordLength(newPassword); err != nil {
		return err
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if rawToken == "" {
		return ErrLinkInvalid
	}
	link, err := s.store.ConsumeLink(ctx, hashToken(rawToken), LinkPurposeResetPassword, s.now())
	if err != nil {
		if errors.Is(err, ErrLinkInvalid) {
			return ErrLinkInvalid
		}
		return fmt.Errorf("consume password reset link: %w", err)
	}
	return s.setPassword(ctx, link.UserID, newHash)
}

// RegisterEmail gives an authenticated emailless account its first address. A taken
// address produces the same success response and moves the information into mail.
func (s *Service) RegisterEmail(ctx context.Context, userID, rawEmail string) error {
	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if user.EmailVerifiedAt != nil {
		return ErrEmailAlreadyVerified
	}
	email, err := normalizedEmail(rawEmail)
	if err != nil {
		return err
	}
	existing, found, err := s.accountForEmail(ctx, email)
	if err != nil {
		return err
	}
	if found && existing.ID != user.ID {
		return s.send(ctx, existing, existingAccountMail(email))
	}
	if err := s.store.SetEmail(ctx, user.ID, email, nil); err != nil {
		if !errors.Is(err, ErrDuplicateUser) {
			return err
		}
		existing, found, loadErr := s.accountForEmail(ctx, email)
		if loadErr != nil {
			return fmt.Errorf("resolve concurrently registered email: %w", loadErr)
		}
		if !found {
			return errors.New("resolve concurrently registered email: duplicate account disappeared")
		}
		return s.send(ctx, existing, existingAccountMail(email))
	}
	user.Email = email
	user.EmailVerifiedAt = nil
	return s.sendVerification(ctx, user)
}

// ChangePassword proves the current password without reclassifying a bad proof as a lost
// session. A Google-only account has no current password and must use the mailed reset path.
func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if user.PasswordHash == "" {
		return ErrPasswordNotSet
	}
	ok, err := s.verify(currentPassword, user.PasswordHash)
	if err != nil {
		slog.Error("stored password hash is unusable during password change", "user_id", user.ID, "err", err)
		return ErrCurrentPasswordWrong
	}
	if !ok {
		return ErrCurrentPasswordWrong
	}
	if err := validatePasswordLength(newPassword); err != nil {
		return err
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.setPassword(ctx, user.ID, newHash)
}

func normalizedEmail(raw string) (string, error) {
	email, err := NormalizeEmail(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidEmail, err)
	}
	return email, nil
}

func validatePasswordLength(password string) error {
	length := utf8.RuneCountInString(password)
	if length < PasswordMinLen {
		return ErrPasswordTooShort
	}
	if length > PasswordMaxLen {
		return ErrPasswordTooLong
	}
	return nil
}

func (s *Service) accountForEmail(ctx context.Context, email string) (User, bool, error) {
	user, err := s.store.GetUser(ctx, email)
	if err == nil {
		return user, true, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return User{}, false, fmt.Errorf("look up account id: %w", err)
	}
	user, err = s.store.GetUserByEmail(ctx, email)
	if err == nil {
		return user, true, nil
	}
	if errors.Is(err, ErrUserNotFound) {
		return User{}, false, nil
	}
	return User{}, false, fmt.Errorf("look up account email: %w", err)
}

func (s *Service) sendVerification(ctx context.Context, user User) error {
	now := s.now()
	if !s.reserveResend(user.Email, now) {
		return nil
	}
	raw, tokenHash, err := NewLinkToken()
	if err != nil {
		s.releaseResend(user.Email, now)
		return err
	}
	linkURL, err := s.verificationLink(raw)
	if err != nil {
		s.releaseResend(user.Email, now)
		return err
	}
	if err := s.store.InvalidateLinks(ctx, user.ID, LinkPurposeVerifyEmail, now); err != nil {
		s.releaseResend(user.Email, now)
		return fmt.Errorf("invalidate verification links: %w", err)
	}
	if err := s.store.CreateLink(ctx, Link{
		TokenHash: tokenHash, UserID: user.ID, Purpose: LinkPurposeVerifyEmail,
		Email: user.Email, ExpiresAt: now.Add(VerifyLinkTTL), CreatedAt: now,
	}); err != nil {
		s.releaseResend(user.Email, now)
		return fmt.Errorf("create verification link: %w", err)
	}
	if err := s.send(ctx, user, verificationMail(user.Email, linkURL)); err != nil {
		s.releaseResend(user.Email, now)
		return err
	}
	return nil
}

func (s *Service) reserveResend(email string, now time.Time) bool {
	s.resendMu.Lock()
	defer s.resendMu.Unlock()
	for address, sentAt := range s.lastResend {
		if !now.Before(sentAt.Add(ResendFloor)) {
			delete(s.lastResend, address)
		}
	}
	if sentAt, exists := s.lastResend[email]; exists && now.Before(sentAt.Add(ResendFloor)) {
		return false
	}
	s.lastResend[email] = now
	return true
}

func (s *Service) releaseResend(email string, reservedAt time.Time) {
	s.resendMu.Lock()
	defer s.resendMu.Unlock()
	if s.lastResend[email].Equal(reservedAt) {
		delete(s.lastResend, email)
	}
}

func (s *Service) runBootstraps(ctx context.Context, userID string) error {
	for _, bootstrap := range s.bootstraps {
		if err := bootstrap(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}

// setPassword is the one mutation path shared by reset and signed-in change. The hash
// is durable before sessions are revoked, so no session is ended unless the replacement
// credential can already authenticate the owner.
func (s *Service) setPassword(ctx context.Context, userID, newHash string) error {
	if err := s.store.UpdatePasswordHash(ctx, userID, newHash); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if err := s.store.DeleteSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("password changed but session revocation failed: %w", err)
	}
	return nil
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
	if errors.Is(err, ErrUserNotFound) {
		if email, normalizeErr := NormalizeEmail(loginID); normalizeErr == nil {
			user, err = s.store.GetUserByEmail(ctx, email)
		}
	}
	switch {
	case errors.Is(err, ErrUserNotFound):
		// The equalizing derivation. Its result is meaningless — running it is the point.
		_, _ = s.verify(password, dummyHash())
		return User{}, "", ErrInvalidCredentials
	case err != nil:
		return User{}, "", fmt.Errorf("load user: %w", err)
	}

	if user.PasswordHash == "" {
		_, _ = s.verify(password, dummyHash())
		return User{}, "", ErrInvalidCredentials
	}
	now := s.now()
	ok, err := s.verify(password, user.PasswordHash)
	if err != nil {
		// A stored hash that will not parse is an operator problem (a hand-edited row,
		// a botched restore). Record it, but answer the client exactly as for a wrong
		// password — the client learns nothing either way.
		slog.Error("stored password hash is unusable", "user_id", user.ID, "err", err)
		return User{}, "", ErrInvalidCredentials
	}
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		return User{}, "", ErrInvalidCredentials
	}
	if !ok {
		s.recordLoginFailure(ctx, user, now)
		return User{}, "", ErrInvalidCredentials
	}
	if err := s.store.ClearLoginFailures(ctx, user.ID); err != nil {
		return User{}, "", fmt.Errorf("clear login failures: %w", err)
	}
	if user.Email != "" && user.EmailVerifiedAt == nil {
		if err := s.sendVerification(ctx, user); err != nil {
			slog.Error("could not resend verification after correct credentials", "user_id", user.ID, "err", err)
		}
		return User{}, "", ErrInvalidCredentials
	}

	return s.issueSession(ctx, user, now)
}

// SignInWithGoogleCode exchanges the browser's one-time authorization code before
// handing the validated facts to the domain join rule. Disabled is explicit so a
// deployment with no Google credentials gives the callback a stable refusal.
func (s *Service) SignInWithGoogleCode(ctx context.Context, code, codeVerifier, redirectURI string) (User, string, error) {
	if s.google == nil {
		return User{}, "", ErrGoogleSignInDisabled
	}
	claims, err := s.google.Exchange(ctx, code, codeVerifier, redirectURI)
	if err != nil {
		return User{}, "", fmt.Errorf("%w: %v", ErrGoogleSignInFailed, err)
	}
	return s.SignInWithGoogle(ctx, claims)
}

// SignInWithGoogle joins on the provider's stable subject first and the normalized
// verified email second. Password lockouts intentionally do not participate: Google
// has just proved the identity without guessing the account password.
func (s *Service) SignInWithGoogle(ctx context.Context, claims GoogleClaims) (User, string, error) {
	if !claims.EmailVerified || strings.TrimSpace(claims.Email) == "" {
		return User{}, "", ErrGoogleEmailUnverified
	}
	email, err := normalizedEmail(claims.Email)
	if err != nil || strings.TrimSpace(claims.Subject) == "" {
		return User{}, "", ErrGoogleEmailUnverified
	}
	claims.Subject = strings.TrimSpace(claims.Subject)

	user, err := s.store.GetUserByGoogleSubject(ctx, claims.Subject)
	if err == nil {
		return s.issueSession(ctx, user, s.now())
	}
	if !errors.Is(err, ErrUserNotFound) {
		return User{}, "", fmt.Errorf("look up google subject: %w", err)
	}

	user, err = s.store.GetUserByEmail(ctx, email)
	found := err == nil
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return User{}, "", fmt.Errorf("look up google email: %w", err)
	}
	now := s.now()
	if found {
		if user.GoogleSubject != "" && user.GoogleSubject != claims.Subject {
			return User{}, "", ErrGoogleAccountMismatch
		}
		if err := s.store.BindGoogleIdentity(ctx, user.ID, claims.Subject, now); err != nil {
			if errors.Is(err, ErrGoogleAccountMismatch) {
				return User{}, "", ErrGoogleAccountMismatch
			}
			return User{}, "", fmt.Errorf("join google identity: %w", err)
		}
		user.GoogleSubject = claims.Subject
		if user.EmailVerifiedAt == nil {
			user.EmailVerifiedAt = &now
		}
		return s.issueSession(ctx, user, now)
	}

	user = User{
		ID: email, Email: email, PasswordHash: "", EmailVerifiedAt: &now,
		GoogleSubject: claims.Subject, Plan: plan.Free, CreatedAt: now,
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		if errors.Is(err, ErrDuplicateUser) {
			return User{}, "", ErrGoogleAccountMismatch
		}
		return User{}, "", fmt.Errorf("create google account: %w", err)
	}
	if err := s.runBootstraps(ctx, user.ID); err != nil {
		return User{}, "", fmt.Errorf("bootstrap google account: %w", err)
	}
	return s.issueSession(ctx, user, now)
}

func (s *Service) issueSession(ctx context.Context, user User, now time.Time) (User, string, error) {
	raw, hashed, err := newSessionToken()
	if err != nil {
		return User{}, "", err
	}

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

// recordLoginFailure preserves the generic credential response even if accounting or
// notification fails. Returning a different wire error here would reveal that the id
// reached an existing account, undoing the dummy-hash path's enumeration protection.
func (s *Service) recordLoginFailure(ctx context.Context, user User, now time.Time) {
	count, err := s.store.RecordLoginFailure(ctx, user.ID, now)
	if err != nil {
		slog.ErrorContext(ctx, "could not record login failure", "user_id", user.ID, "err", err)
		return
	}
	if count < LockThreshold || user.Email == "" || user.EmailUnreachableAt != nil {
		return
	}
	if err := s.send(ctx, user, lockNoticeMail(user.Email, now.Add(LockDuration))); err != nil {
		slog.ErrorContext(ctx, "could not send login lock notice", "user_id", user.ID, "err", err)
	}
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

// CreatedAt returns the account creation instant used as its monthly-credit anchor while
// it has no subscription. The store query behind it deliberately loads no credential data.
func (s *Service) CreatedAt(ctx context.Context, userID string) (time.Time, error) {
	return s.store.GetUserCreatedAt(ctx, userID)
}

// Account returns the client-visible identity fields for an authenticated account.
func (s *Service) Account(ctx context.Context, userID string) (User, error) {
	return s.store.GetUser(ctx, userID)
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

	// An upgrade owes the difference for the cycle already running (QUOTA-35). The tier is
	// written first so a failure here leaves an account that is on the tier it paid for but
	// short of the credits, which the error names; the reverse would hand out credits for a
	// tier the account never reached. Money will make this seam BILLING's problem to close.
	owed := plan.MonthlyCredits(target) - plan.MonthlyCredits(current)
	tops := owed > 0 && !plan.Unlimited(target) && !plan.Unlimited(current)
	if tops && s.topUp == nil {
		return fmt.Errorf("set plan %s: no monthly top-up is wired", target)
	}
	if err := s.store.SetUserPlan(ctx, userID, target); err != nil {
		return err
	}
	if !tops {
		return nil
	}
	if err := s.topUp(ctx, userID, owed); err != nil {
		return fmt.Errorf("account is on %s but its %d-credit top-up failed: %w", target, owed, err)
	}
	return nil
}

// AssignTier is billing's plan write. Unlike the operator SetUserPlan path it grants no
// credits: billing coordinates its own charge and usage lot, so a hidden top-up here would
// duplicate that grant.
func (s *Service) AssignTier(ctx context.Context, userID string, target plan.Plan) error {
	if !target.Valid() {
		return fmt.Errorf("unknown plan %q", target)
	}
	current, err := s.store.GetUserPlan(ctx, userID)
	if err != nil {
		return err
	}
	if current == target {
		return nil
	}
	return s.store.SetUserPlan(ctx, userID, target)
}

func (s *Service) TierOf(ctx context.Context, userID string) (plan.Plan, error) {
	return s.store.GetUserPlan(ctx, userID)
}

// VerifiedEmail returns only an address that can receive billing notices. Operator-created
// legacy accounts remain valid but answer ok=false until they register and verify one.
func (s *Service) VerifiedEmail(ctx context.Context, userID string) (string, bool, error) {
	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return "", false, err
	}
	ok := user.Email != "" && user.EmailVerifiedAt != nil && user.EmailUnreachableAt == nil
	return user.Email, ok, nil
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

// CreateUser provisions an account on a tier from the operator command. Unlike Signup,
// the tier is an operator argument; it is never accepted from a public request.
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
