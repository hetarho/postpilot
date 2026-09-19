package experiment

import (
	"context"
	"log/slog"
	"time"
)

type Sweeper struct {
	retention RunRetention
	now       func() time.Time
}

func NewSweeper(retention RunRetention) *Sweeper {
	return &Sweeper{retention: retention, now: time.Now}
}

func (s *Sweeper) Sweep(ctx context.Context) (int64, error) {
	return s.retention.PurgeExpired(ctx, s.now())
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
