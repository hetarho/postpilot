package spoken

import (
	"context"
	"log/slog"
	"time"
)

// Five minutes exceeds all three bounded audition uploads. Intents survive
// account deletion and failed deletion is retried on the next sweep.
func (s *Service) RunCleanup(ctx context.Context, interval, minAge time.Duration) {
	if interval <= 0 {
		panic("spoken cleanup interval required")
	}
	if minAge < 5*time.Minute {
		minAge = 5 * time.Minute
	}
	timer := time.NewTicker(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			sweep, cancel := context.WithTimeout(ctx, time.Minute)
			if err := s.Cleanup(sweep, s.now().Add(-minAge)); err != nil && ctx.Err() == nil {
				slog.Warn("spoken audio cleanup pending")
			}
			cancel()
		}
	}
}
