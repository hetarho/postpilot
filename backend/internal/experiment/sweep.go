package experiment

import (
	"context"
	"log/slog"
	"time"
)

type Sweeper struct {
	store Store
	now   func() time.Time
}

func NewSweeper(store Store) *Sweeper { return &Sweeper{store: store, now: time.Now} }

func (s *Sweeper) Sweep(ctx context.Context) (int64, error) {
	return s.store.PurgeExpired(ctx, s.now())
}

func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := s.Sweep(ctx)
			if err != nil {
				slog.Warn("experiment content sweep failed", "err", err)
			} else if count > 0 {
				slog.Info("experiment content purged", "count", count)
			}
		}
	}
}
