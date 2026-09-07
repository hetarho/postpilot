package auth

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// MonthlyTopUp raises an account's current monthly credit grant. Declared here by its
// consumer and implemented by the usage context, which owns credit_lots: a tier lives in
// this context's table, the credits it grants do not.
type MonthlyTopUp func(ctx context.Context, userID string, credits int) error

// AccountBootstrap installs another context's idempotent defaults after an account row
// exists. The auth context owns the lifecycle and the composition root supplies the work.
type AccountBootstrap func(ctx context.Context, userID string) error

// Mail is deliberately text-only: transactional messages need no product-generated HTML,
// and every delivery adapter receives the same final bilingual body.
type Mail struct {
	To      string
	Subject string
	Text    string
}

// Mailer is owned by auth, its consumer. Delivery vendors stay behind this seam.
type Mailer interface {
	Send(ctx context.Context, m Mail) error
}

// ErrRecipientRejected is the one provider outcome the domain handles specially. It marks
// the address unreachable so later transactional events do not retry it forever.
var ErrRecipientRejected = errors.New("mail recipient rejected")

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). The implementation lives in auth/store and is injected by
// cmd/api; nothing here knows it is SQL.
//
// Implementations must translate "not found" into ErrUserNotFound / ErrNoSession —
// the service branches on those, never on a driver error.
type Store interface {
	CreateUser(ctx context.Context, u User) error
	GetUser(ctx context.Context, id string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	SetEmail(ctx context.Context, id, email string, verifiedAt *time.Time) error
	MarkEmailVerified(ctx context.Context, id string, at time.Time) error
	MarkEmailUnreachable(ctx context.Context, id string, at time.Time) error
	UpdatePasswordHash(ctx context.Context, id, passwordHash string) error
	// GetUserPlan is deliberately narrower than GetUser: the interceptor resolves the
	// acting plan on every authenticated request, and loading a password hash that often
	// widens the blast radius of any log or dump for a value nothing on that path reads.
	GetUserPlan(ctx context.Context, id string) (plan.Plan, error)
	// GetUserCreatedAt is the equally narrow account anchor read used by the credit ledger.
	GetUserCreatedAt(ctx context.Context, id string) (time.Time, error)
	SetUserPlan(ctx context.Context, id string, p plan.Plan) error
	ListUsers(ctx context.Context) ([]User, error)

	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, hashedToken string) (Session, error)
	DeleteSession(ctx context.Context, hashedToken string) error
	DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error)
	DeleteSessionsForUser(ctx context.Context, userID string) error

	CreateLink(ctx context.Context, link Link) error
	ConsumeLink(ctx context.Context, tokenHash string, purpose LinkPurpose, now time.Time) (Link, error)
	InvalidateLinks(ctx context.Context, userID string, purpose LinkPurpose, now time.Time) error
}
