package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type Control interface {
	Claim(context.Context, clip.MediaWorkerProfile) (*clip.MediaWork, error)
	Renew(context.Context, clip.MediaLeaseCredentials, int) (time.Time, error)
	Complete(context.Context, clip.MediaLeaseCredentials, string) error
	Fail(context.Context, clip.MediaLeaseCredentials, clip.MediaFailure) error
}
type Execution interface {
	Execute(context.Context, clip.MediaWork) (string, error)
}
type Loop struct {
	Control      Control
	Executor     Execution
	Profile      clip.MediaWorkerProfile
	DrainTimeout time.Duration
	PollDelay    func() time.Duration
}

// Run stops claims immediately on shutdown, keeping the current lease alive for
// at most DrainTimeout. Once that expires the subprocess context is cancelled.
func (l Loop) Run(ctx context.Context) error {
	if l.DrainTimeout <= 0 || l.PollDelay == nil || !l.Profile.Compatible() {
		return clip.ErrInvalid
	}
	op := clip.MediaPrepare
	for ctx.Err() == nil {
		profile := l.Profile
		profile.Operation = op
		work, err := l.Control.Claim(ctx, profile)
		if errors.Is(err, clip.ErrMediaUnauthenticated) || errors.Is(err, clip.ErrMediaIncompatible) {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		if work != nil {
			current, cancel := context.WithCancel(context.Background())
			drained := make(chan struct{})
			go func() {
				defer close(drained)
				select {
				case <-current.Done():
					return
				case <-ctx.Done():
				}
				timer := time.NewTimer(l.DrainTimeout)
				defer timer.Stop()
				select {
				case <-current.Done():
				case <-timer.C:
					cancel()
				}
			}()
			if err = l.RunLease(current, *work); err != nil {
				slog.Warn("media attempt ended without a completion receipt")
			}
			cancel()
			<-drained
		}
		// Alternate operations even under a continuously full queue.
		if op == clip.MediaPrepare {
			op = clip.MediaRender
		} else {
			op = clip.MediaPrepare
		}
		if work == nil {
			if !pause(ctx, l.PollDelay()) {
				return nil
			}
		}
	}
	return nil
}
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// RunLease uses only local monotonic deadlines. A failed heartbeat cannot move
// the expiry forward, and the watchdog stays live while the HTTP call is pending.
func (l Loop) RunLease(parent context.Context, w clip.MediaWork) error {
	margin := min(time.Second, w.LeaseRemaining/10)
	if w.HeartbeatAfter <= 0 || w.StageRemaining <= 0 || !w.LeaseExpiresAt.Add(-margin).After(time.Now()) {
		return clip.ErrMediaLeaseLost
	}
	stage, cancelStage := context.WithTimeout(parent, w.StageRemaining)
	defer cancelStage()
	ctx, cancel := context.WithCancelCause(stage)
	defer cancel(nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		cutoff := w.LeaseExpiresAt.Add(-margin)
		expiry := time.NewTimer(time.Until(cutoff))
		defer expiry.Stop()
		heartbeat := time.NewTimer(min(w.HeartbeatAfter, time.Until(w.LeaseExpiresAt.Add(-margin))/2))
		defer heartbeat.Stop()
		type renewal struct {
			expires time.Time
			err     error
		}
		replies := make(chan renewal, 1)
		var pending bool
		for {
			select {
			case <-ctx.Done():
				return
			case <-expiry.C:
				cancel(clip.ErrMediaLeaseLost)
				return
			case <-heartbeat.C:
				if !pending {
					pending = true
					go func() { at, err := l.Control.Renew(ctx, w.Credentials, 0); replies <- renewal{at, err} }()
				}
			case r := <-replies:
				if !cutoff.After(time.Now()) {
					cancel(clip.ErrMediaLeaseLost)
					return
				}
				pending = false
				if r.err == nil {
					if !r.expires.Add(-margin).After(time.Now()) {
						cancel(clip.ErrMediaLeaseLost)
						return
					}
					cutoff = r.expires.Add(-margin)
					expiry.Reset(time.Until(cutoff))
				} else if !errors.Is(r.err, clip.ErrMediaUnavailable) && !errors.Is(r.err, context.DeadlineExceeded) {
					cancel(r.err)
					return
				}
				heartbeat.Reset(w.HeartbeatAfter)
			}
		}
	}()
	result, err := l.Executor.Execute(ctx, w)
	if ctx.Err() != nil {
		err = context.Cause(ctx)
	}
	if err == nil {
		err = retryControl(ctx, func() error { return l.Control.Complete(ctx, w.Credentials, result) })
	} else if ctx.Err() == nil {
		code := clip.MediaFailureInternal
		if errors.Is(err, clip.ErrInvalid) || errors.Is(err, clip.ErrInvalidMedia) || errors.Is(err, clip.ErrMediaIncompatible) {
			code = clip.MediaFailureInvalidInput
		}
		if errors.Is(err, clip.ErrMediaUnavailable) {
			code = clip.MediaFailureWorkerLost
		}
		_ = retryControl(ctx, func() error { return l.Control.Fail(ctx, w.Credentials, code) })
	}
	cancel(nil)
	<-done
	return err
}
func retryControl(ctx context.Context, fn func() error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		err = fn()
		if !errors.Is(err, clip.ErrMediaUnavailable) && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if !pause(ctx, time.Duration(i+1)*200*time.Millisecond) {
			return ctx.Err()
		}
	}
	return err
}
