package job

import (
	"context"
	"time"
)

// Store is the persistence behavior the queue needs. SQL rows stop in store/.
type Store interface {
	Insert(ctx context.Context, found Job) error
	PickNextQueued(ctx context.Context, now time.Time) (Job, error)
	UpdateProgress(ctx context.Context, id, stage string, done, total int, now time.Time) error
	Finish(ctx context.Context, id, status, message string, now time.Time) error
	SweepRunning(ctx context.Context, message string, now time.Time) (int64, error)
	ActiveForPost(ctx context.Context, slug string) (*Job, error)
	ActiveForPostUser(ctx context.Context, slug, userID string) (*Job, error)
	ActiveForUserKind(ctx context.Context, userID, kind string) (*Job, error)
	ActiveModelExperiment(ctx context.Context, experimentID string) (*Job, error)
	GetByID(ctx context.Context, id string) (Job, error)
}
