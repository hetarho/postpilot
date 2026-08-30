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
	// the first release to one browser/Hermes publication at a time.
	Permit chan struct{}
}

func (s Supervisor) Run(ctx context.Context) error {
	interval := s.PollInterval
	if interval <= 0 {
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
		if connect.CodeOf(err) == connect.CodeNotFound {
			backoff = interval
		} else if connect.CodeOf(err) == connect.CodeUnauthenticated {
			return errors.New("publishing token was revoked")
		} else if backoff < time.Minute {
			backoff *= 2
		}
		jitter := time.Duration(rand.Int64N(int64(backoff / 4)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff + jitter):
		}
	}
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
