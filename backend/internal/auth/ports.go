package auth

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// Store is the persistence this context needs, declared here by its consumer
// (ARCHITECTURE §2.2). The implementation lives in auth/store and is injected by
// cmd/api; nothing here knows it is SQL.
//
// Implementations must translate "not found" into ErrUserNotFound / ErrNoSession —
// the service branches on those, never on a driver error.
type Store interface {
	CreateUser(ctx context.Context, u User) error
	GetUser(ctx context.Context, id string) (User, error)
	// GetUserPlan is deliberately narrower than GetUser: the interceptor resolves the
	// acting plan on every authenticated request, and loading a password hash that often
	// widens the blast radius of any log or dump for a value nothing on that path reads.
	GetUserPlan(ctx context.Context, id string) (plan.Plan, error)
	SetUserPlan(ctx context.Context, id string, p plan.Plan) error
	ListUsers(ctx context.Context) ([]User, error)

	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, hashedToken string) (Session, error)
	DeleteSession(ctx context.Context, hashedToken string) error
	DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error)
}
