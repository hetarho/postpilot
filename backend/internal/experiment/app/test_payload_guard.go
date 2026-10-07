package app

import (
	"context"
	"database/sql"
)

type TestPayloadAvailability interface {
	TestPayloadAvailable(context.Context, string, string, string, uint64) (bool, error)
}

// TestPayloadGuard binds the experiment owner's published read behavior to the
// exact post publication transaction, without querying either owner's tables.
type TestPayloadGuard struct {
	owned func(*sql.Tx) TestPayloadAvailability
}

func NewTestPayloadGuard(owned func(*sql.Tx) TestPayloadAvailability) *TestPayloadGuard {
	if owned == nil {
		panic("experiment/app: transactional payload availability is required")
	}
	return &TestPayloadGuard{owned: owned}
}

func (a *TestPayloadGuard) TestPayloadAvailable(ctx context.Context, tx *sql.Tx, user, test, winner string, fence uint64) (bool, error) {
	return a.owned(tx).TestPayloadAvailable(ctx, user, test, winner, fence)
}
