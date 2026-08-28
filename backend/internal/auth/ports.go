package auth

import (
	"context"
	"time"
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

	CreateSession(ctx context.Context, s Session) error
	GetSession(ctx context.Context, hashedToken string) (Session, error)
	DeleteSession(ctx context.Context, hashedToken string) error
	DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error)
}
