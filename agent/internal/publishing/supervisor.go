package publishing

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"connectrpc.com/connect"
	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
)

type Claimer interface {
	Claim(context.Context) (*postpilotv1.ClaimPublishJobResponse, error)
}
type JobExecutor interface {
	Execute(context.Context, *postpilotv1.ClaimPublishJobResponse) error
}

type Supervisor struct {
	Client       Claimer
	Executor     JobExecutor
	PollInterval time.Duration
	Logger       *slog.Logger
	// Permit is shared by every connection. Holding it across claim+execute keeps
	// the first release to one deterministic browser publication at a time.
	Permit chan struct{}
}

func (s Supervisor) Run(ctx context.Context) error {
	interval := s.PollInterval
	if interval <= 0 {
		// Mirrors config.PollInterval, which is what callers actually pass; keep the
		// two in step so a change to the poll rate is not half-applied.
		interval = 5 * time.Second
	}
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	backoff := interval
	for {
		if !s.acquire(ctx) {
			return ctx.Err()
		}
		claim, err := s.Client.Claim(ctx)
		if err == nil {
			backoff = interval
			if executeErr := s.Executor.Execute(ctx, claim); executeErr != nil {
				logger.Error("publish job stopped", "job_id", claim.GetJob().GetId(), "error", executeErr)
			}
			s.release()
			continue
		}
		s.release()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if connect.CodeOf(err) == connect.CodeUnauthenticated {
			return errors.New("publishing token was revoked")
		}
		backoff = backoffAfter(backoff, interval, err)
		jitter := time.Duration(rand.Int64N(int64(backoff / 4)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff + jitter):
		}
	}
}

// backoffAfter applies PUBLISH-29's polling rule to one failed claim. An empty queue is
// the ordinary answer and returns to the base interval, so a Mac that slept through a
// quiet night resumes at full cadence; any other transient failure doubles the wait, and
// the doubling stops once the wait has passed a minute.
func backoffAfter(current, interval time.Duration, err error) time.Duration {
	if connect.CodeOf(err) == connect.CodeNotFound {
		return interval
	}
	if current < time.Minute {
		return current * 2
	}
	return current
}

func (s Supervisor) acquire(ctx context.Context) bool {
	if s.Permit == nil {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-s.Permit:
		return true
	}
}

func (s Supervisor) release() {
	if s.Permit != nil {
		s.Permit <- struct{}{}
	}
}
