package app

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/job"
)

type MediaRecoveryTx interface {
	MediaRecoveryState(context.Context, string) (clip.MediaRecovery, error)
	SetMediaRecoveryState(context.Context, string, clip.MediaStageState, clip.MediaFailure, time.Time) error
	StopMediaAttempt(context.Context, string, string, time.Time) error
	RetireMediaArtifacts(context.Context, string, string, bool) error
	MarkMediaReconciled(context.Context, string, time.Time) error
}
type MediaRecoveryStore interface {
	MediaRecoveryStages(context.Context, string) ([]clip.MediaStage, error)
	RetireSupersededMediaResults(context.Context) error
	DueMediaDeletions(context.Context, time.Time) ([]clip.MediaDeletion, error)
	RemoveMediaDeletion(context.Context, string) error
	QueueOrphanMediaDeletion(context.Context, string, time.Time) error
}
type MediaRecoveryJobs interface {
	AcknowledgeWaitCancellation(context.Context, string, string) (bool, error)
	FailWait(context.Context, string, string, job.Failure) (bool, error)
}
type MediaRecoveryWaits interface {
	WaitingContinuations(context.Context, string, string) ([]job.Continuation, error)
}
type MediaRecoveryObjects interface {
	Delete(context.Context, string) error
	ListMediaOutputs(context.Context) ([]clip.StoredObject, error)
}

type MediaReconciler struct {
	writer                  *sql.DB
	bind                    Binder
	store                   MediaRecoveryStore
	waits                   MediaRecoveryWaits
	queue                   MediaRecoveryJobs
	objects                 MediaRecoveryObjects
	now                     func() time.Time
	grace                   time.Duration
	mu                      sync.Mutex
	stageCursor, waitCursor string
}

func NewMediaReconciler(writer *sql.DB, bind Binder, store MediaRecoveryStore, waits MediaRecoveryWaits, queue MediaRecoveryJobs, objects MediaRecoveryObjects, grace time.Duration, now func() time.Time) *MediaReconciler {
	if now == nil {
		now = time.Now
	}
	if writer == nil || bind == nil || store == nil || waits == nil || queue == nil || objects == nil || grace <= 0 {
		panic("media recovery requires bounded owned persistence and object access")
	}
	return &MediaReconciler{writer: writer, bind: bind, store: store, waits: waits, queue: queue, objects: objects, grace: grace, now: now}
}

// Reconcile performs at most 100 short stage transactions and 100 wait checks.
// No object I/O holds the SQLite writer. Rotating cursors avoid starvation.
func (r *MediaReconciler) Reconcile(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stages, err := r.store.MediaRecoveryStages(ctx, r.stageCursor)
	if err != nil {
		return err
	}
	for _, stage := range stages {
		if err = r.reconcileStage(ctx, stage.ID); err != nil {
			return err
		}
		r.stageCursor = stage.ID
	}
	if len(stages) < 100 {
		r.stageCursor = ""
	}
	waits, err := r.waits.WaitingContinuations(ctx, "clip-media:", r.waitCursor)
	if err != nil {
		return err
	}
	for _, wait := range waits {
		var missing, cancelled bool
		err = WriteTx(ctx, r.writer, r.bind, func(p Ports) error {
			stage, err := p.Recovery.MediaRecoveryState(ctx, strings.TrimPrefix(wait.WaitKey, "clip-media:"))
			if err != nil && !errors.Is(err, clip.ErrNotFound) {
				return err
			}
			missing = errors.Is(err, clip.ErrNotFound) || stage.Stage.ParentJobID != wait.JobID
			if !missing {
				return nil
			}
			j, err := p.Jobs.GetByID(ctx, wait.JobID)
			if err != nil {
				return err
			}
			cancelled = j.CancelRequestedAt != nil
			return nil
		})
		if err != nil {
			return err
		}
		if missing {
			if cancelled {
				_, err = r.queue.AcknowledgeWaitCancellation(ctx, wait.JobID, wait.WaitKey)
			} else {
				_, err = r.queue.FailWait(ctx, wait.JobID, wait.WaitKey, job.Failure{Reason: "CLIP_MEDIA_UNAVAILABLE"})
			}
			if err != nil {
				return err
			}
		}
		r.waitCursor = wait.JobID
	}
	if len(waits) < 100 {
		r.waitCursor = ""
	}
	return nil
}

func (r *MediaReconciler) reconcileStage(ctx context.Context, id string) error {
	var ack, parent string
	err := WriteTx(ctx, r.writer, r.bind, func(p Ports) error {
		now := r.now().UTC()
		state, err := p.Recovery.MediaRecoveryState(ctx, id)
		if errors.Is(err, clip.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		s := state.Stage
		parent = s.ParentJobID
		j, err := p.Jobs.GetByID(ctx, s.ParentJobID)
		if err != nil && !errors.Is(err, job.ErrNotFound) {
			return err
		}
		terminal := errors.Is(err, job.ErrNotFound) || job.Terminal(j.Status)
		cancelled := !terminal && j.CancelRequestedAt != nil
		stop := func(outcome string) error {
			if s.CurrentAttemptID != "" && state.Outcome == "" && !state.LeaseExpiresAt.After(now) {
				return p.Recovery.StopMediaAttempt(ctx, s.CurrentAttemptID, outcome, now)
			}
			return nil
		}
		if terminal || cancelled {
			if err = p.Recovery.SetMediaRecoveryState(ctx, id, clip.MediaCancelled, "", time.Time{}); err != nil {
				return err
			}
			if err = stop("cancelled"); err != nil {
				return err
			}
			if err = p.Recovery.RetireMediaArtifacts(ctx, id, s.CurrentAttemptID, true); err != nil {
				return err
			}
			if state.Stopped(now) {
				if cancelled {
					ack = MediaWaitKey(id)
				} else {
					return p.Recovery.MarkMediaReconciled(ctx, id, now)
				}
			}
			return nil
		}
		// Keep an accepted handoff alive while the API consumes it. No model
		// handler is invoked by recovery, including after an uncertain crash.
		if s.State != clip.MediaSucceeded {
			task, e := mediacodec.DecodeTask(s.Payload)
			if e == nil {
				e = authorizeMediaParent(ctx, p, s, task, now)
			}
			if e != nil && !errors.Is(e, clip.ErrInvalid) && !errors.Is(e, clip.ErrInvalidMedia) && !errors.Is(e, clip.ErrMediaIncompatible) && !errors.Is(e, clip.ErrMediaLeaseLost) && !errors.Is(e, clip.ErrMediaCancelled) && !errors.Is(e, clip.ErrSourceState) && !errors.Is(e, clip.ErrNotFound) {
				return e
			}
			failure := clip.MediaFailure("")
			switch {
			case e != nil:
				failure = clip.MediaFailureInvalidInput
			case !s.DeadlineAt.After(now):
				failure = clip.MediaFailureDeadlineExceeded
			case s.AttemptCount == 0 && !s.QueueDeadlineAt.After(now):
				failure = clip.MediaFailureWaitExpired
			case s.State == clip.MediaRunning && state.Stopped(now) && s.AttemptCount >= s.Limits.MaxAttempts:
				failure = clip.MediaFailureAttemptsExhausted
			}
			if failure != "" && s.State != clip.MediaFailed && s.State != clip.MediaCancelled {
				s.State, s.Failure = clip.MediaFailed, failure
				if err = p.Recovery.SetMediaRecoveryState(ctx, id, s.State, failure, time.Time{}); err != nil {
					return err
				}
			} else if s.State == clip.MediaRunning && state.Stopped(now) {
				s.State = clip.MediaQueued
				if err = p.Recovery.SetMediaRecoveryState(ctx, id, s.State, clip.MediaFailureWorkerLost, time.Time{}); err != nil {
					return err
				}
			}
			if err = stop("expired"); err != nil {
				return err
			}
		}
		if (s.State == clip.MediaQueued || s.State == clip.MediaRunning) && j.Stage != clip.MediaJobStage(s) {
			if err = p.Waits.UpdateProgress(ctx, s.ParentJobID, clip.MediaJobStage(s), 0, 0, now); err != nil {
				return err
			}
		}
		all := s.State != clip.MediaRunning && s.State != clip.MediaSucceeded
		if err = p.Recovery.RetireMediaArtifacts(ctx, id, s.CurrentAttemptID, all); err != nil {
			return err
		}
		if s.State == clip.MediaSucceeded || (s.State == clip.MediaFailed || s.State == clip.MediaCancelled) && state.Stopped(now) {
			_, err = p.Waits.Wake(ctx, s.ParentJobID, MediaWaitKey(id), now)
		}
		return err
	})
	if err == nil && ack != "" {
		_, err = r.queue.AcknowledgeWaitCancellation(ctx, parent, ack)
	}
	return err
}

// Deletion waits beyond every issued PUT's expiry plus the orphan grace, so an
// upload already in flight cannot recreate a key after its intent disappeared.
func (r *MediaReconciler) Cleanup(ctx context.Context) error {
	if err := r.store.RetireSupersededMediaResults(ctx); err != nil {
		return err
	}
	rows, err := r.store.DueMediaDeletions(ctx, r.now().Add(-r.grace))
	if err != nil {
		return err
	}
	var pending error
	for _, row := range rows {
		if !mediaOutputKey(row.Key) {
			pending = errors.Join(pending, clip.ErrInvalid)
			continue
		}
		if err = r.objects.Delete(ctx, row.Key); err != nil {
			pending = errors.New("media artifact deletion pending")
			if ctx.Err() != nil {
				break
			}
			continue
		}
		if err = r.store.RemoveMediaDeletion(ctx, row.Key); err != nil {
			return err
		}
	}
	return pending
}
func mediaOutputKey(key string) bool {
	for _, prefix := range []string{clip.MediaAnalysisPrefix, clip.ResultPrefix} {
		if suffix, ok := strings.CutPrefix(key, prefix); ok {
			return len(strings.Split(suffix, "/")) == 4
		}
	}
	return false
}
func (r *MediaReconciler) SweepOrphans(ctx context.Context) error {
	objects, err := r.objects.ListMediaOutputs(ctx)
	if err != nil {
		return err
	}
	cutoff := r.now().Add(-r.grace)
	for _, object := range objects {
		if mediaOutputKey(object.Key) && !object.Modified.IsZero() && object.Modified.Before(cutoff) {
			if err = r.store.QueueOrphanMediaDeletion(ctx, object.Key, r.now()); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *MediaReconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pass, cancel := context.WithTimeout(ctx, 4*time.Second)
			err := r.Reconcile(pass)
			if err == nil {
				err = r.Cleanup(pass)
			}
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("media recovery will retry")
			}
		}
	}
}

// A bucket listing is an infrequent orphan pass, never part of API boot or
// the five-second lease loop.
func (r *MediaReconciler) RunOrphans(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pass, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := r.SweepOrphans(pass)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("media orphan cleanup will retry")
			}
		}
	}
}
