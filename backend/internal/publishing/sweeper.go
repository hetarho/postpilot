package publishing

import (
	"context"
	"log/slog"
	"time"
)

type Sweeper struct {
	service       *Service
	minAge        time.Duration
	recoveryEvery time.Duration
}

func NewSweeper(service *Service, minAge, leaseTTL time.Duration) *Sweeper {
	recoveryEvery := leaseTTL / 2
	if recoveryEvery <= 0 {
		recoveryEvery = time.Second
	}
	return &Sweeper{service: service, minAge: minAge, recoveryEvery: recoveryEvery}
}

func (s *Sweeper) Run(ctx context.Context, orphanInterval time.Duration) {
	recoveryTicker := time.NewTicker(s.recoveryEvery)
	orphanTicker := time.NewTicker(orphanInterval)
	defer recoveryTicker.Stop()
	defer orphanTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-recoveryTicker.C:
			if _, _, err := s.service.RecoverExpired(ctx); err != nil {
				slog.Warn("publishing lease recovery failed", "err", err)
			}
			// Object deletion is retryable housekeeping, not part of the durable lease
			// transition. In particular, an R2 outage must never block API startup.
			if err := s.service.CleanupTerminals(ctx); err != nil {
				slog.Warn("publishing terminal cleanup failed", "err", err)
			}
		case <-orphanTicker.C:
			if removed, err := s.service.SweepOrphans(ctx, s.minAge); err != nil {
				slog.Warn("publishing orphan sweep failed", "err", err)
			} else if removed > 0 {
				slog.Info("publishing orphan objects removed", "count", removed)
			}
		}
	}
}
