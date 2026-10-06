package job

import (
	"context"
	"errors"
)

// LatestOwnedStore is a narrow optional result-discovery port for account-owned work.
// It preserves the generic Store contract for queues with no account-owned proposals.
type LatestOwnedStore interface {
	LatestOwnedKind(context.Context, string, string, string) (*Job, error)
}

func (q *Queue) LatestOwnedKind(ctx context.Context, userID, kind, status string) (*Job, error) {
	store, ok := q.store.(LatestOwnedStore)
	if !ok {
		return nil, errors.New("job store does not support owned result discovery")
	}
	found, err := store.LatestOwnedKind(ctx, userID, kind, status)
	if err != nil || found == nil {
		return nil, err
	}
	if found.UserID != userID || found.Kind != kind || len(found.Subjects) != 0 {
		return nil, ErrNotFound
	}
	return found, nil
}
