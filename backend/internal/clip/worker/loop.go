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
	if l.Profile.Operation == clip.MediaVerifyAnalysis {
		op = clip.MediaVerifyAnalysis
	}
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
		// Take turns between the operations even under a continuously full queue.
		op = nextOperation(op)
		if work == nil {
			if !pause(ctx, l.PollDelay()) {
				return nil
			}
		}
	}
	return nil
}

// nextOperation is the claim order: preparation, a browser render's sampling,
// which a page is waiting on, then a server render.
func nextOperation(op clip.MediaOperation) clip.MediaOperation {
	switch op {
	case clip.MediaVerifyAnalysis:
		return clip.MediaVerifyAnalysis
	case clip.MediaPrepare:
		return clip.MediaSample
	case clip.MediaSample:
		return clip.MediaRender
	}
	return clip.MediaPrepare
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
					go func() {
						progress := 0
						if p, ok := l.Executor.(interface{ Progress() int }); ok {
							progress = p.Progress()
						}
						at, err := l.Control.Renew(ctx, w.Credentials, progress)
						replies <- renewal{at, err}
					}()
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
	started := time.Now()
	result, err := l.Executor.Execute(ctx, w)
	outcome := "ok"
	if err != nil {
		outcome = "failed"
	}
	// Only the bounded operation name and numeric execution timing escape.
	if w.Operation == clip.MediaPrepare || w.Operation == clip.MediaRender || w.Operation == clip.MediaSample {
		slog.Info("clip worker execution", "operation", w.Operation, "outcome", outcome, "elapsed_ms", time.Since(started).Milliseconds())
	}
	if ctx.Err() != nil {
		err = context.Cause(ctx)
	}
	if err == nil {
		err = retryControl(ctx, func() error { return l.Control.Complete(ctx, w.Credentials, result) })
		if ctx.Err() != nil {
			err = context.Cause(ctx)
		}
		if err != nil && !errors.Is(err, clip.ErrMediaUnavailable) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, clip.ErrMediaCancelled) && !errors.Is(err, clip.ErrMediaLeaseLost) {
			// A permanent receipt rejection is an output failure, not worker
			// disappearance that would silently re-encode the same bad result.
			report, stop := context.WithTimeout(context.WithoutCancel(parent), 3*time.Second)
			_ = l.Control.Fail(report, w.Credentials, clip.MediaFailureInvalidOutput)
			stop()
			cancel(nil)
			<-done
			return err
		}
	}
	if err != nil && !errors.Is(err, clip.ErrMediaLeaseLost) {
		code := clip.MediaFailureInternal
		if errors.Is(err, clip.ErrInvalid) || errors.Is(err, clip.ErrInvalidMedia) || errors.Is(err, clip.ErrMediaIncompatible) {
			code = clip.MediaFailureInvalidInput
		}
		if errors.Is(err, clip.ErrMediaUnavailable) {
			code = clip.MediaFailureWorkerLost
		}
		if errors.Is(err, context.Canceled) {
			code = clip.MediaFailureWorkerLost
		}
		if errors.Is(err, context.DeadlineExceeded) {
			// A bounded HTTP request can time out while the frozen stage still
			// has time left. Only the stage deadline is a terminal timeout.
			code = clip.MediaFailureWorkerLost
			if errors.Is(stage.Err(), context.DeadlineExceeded) {
				code = clip.MediaFailureDeadlineExceeded
			}
		}
		if errors.Is(err, clip.ErrMediaCancelled) {
			code = clip.MediaFailureCancelled
		}
		switch {
		case errors.Is(err, clip.ErrWorkspaceLimit):
			code = clip.MediaFailureWorkspaceLimit
		case errors.Is(err, clip.ErrInputTooLarge):
			code = clip.MediaFailureInputTooLarge
		case errors.Is(err, clip.ErrAnalysisTooLarge):
			code = clip.MediaFailureAnalysisTooLarge
		}
		// Execute has returned and reaped its descendants before acknowledging
		// cancellation. Its cancelled context must not suppress that receipt.
		report, stop := context.WithTimeout(context.WithoutCancel(parent), 3*time.Second)
		_ = retryControl(report, func() error { return l.Control.Fail(report, w.Credentials, code) })
		stop()
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
