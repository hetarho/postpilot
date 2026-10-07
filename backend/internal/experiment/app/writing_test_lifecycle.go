package app

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/job"
)

type WritingTestLifecycleStore interface {
	PurgeWritingTestPost(context.Context, string, string) error
	PurgeExpiredWritingTests(context.Context, time.Time) (int64, error)
	WritingTestExecutionsForPost(context.Context, string, string) ([]experiment.TestExecutionFence, error)
	ListCancelledUnsettledTestExecutions(context.Context) ([]experiment.TestExecutionFence, error)
	RecoverInterruptedTests(context.Context) error
}
type WritingTestLifecycleJobs interface {
	CancelWritingTestJob(context.Context, string, string) error
	RecoverSettlements(context.Context) error
	ReconcileTerminalSettlements(context.Context) error
	ReconcileCancelledSettlements(context.Context, []experiment.TestExecutionFence) error
}
type WritingTestLifecycle struct {
	legacy experiment.RunRetention
	store  WritingTestLifecycleStore
	jobs   WritingTestLifecycleJobs
}

func NewWritingTestLifecycle(legacy experiment.RunRetention, store WritingTestLifecycleStore, jobs WritingTestLifecycleJobs) *WritingTestLifecycle {
	if legacy == nil || store == nil || jobs == nil {
		panic("experiment/app: retained and current lifecycle and job ports are required")
	}
	return &WritingTestLifecycle{legacy: legacy, store: store, jobs: jobs}
}

// PurgePost closes the owned callback fence before cancelling external work or
// deleting the source. A later retry can still find cancellation metadata after
// the private source reference has been cleared.
func (s *WritingTestLifecycle) PurgePost(ctx context.Context, user, slug string) error {
	fences, err := s.store.WritingTestExecutionsForPost(ctx, user, slug)
	if err != nil {
		return err
	}
	if err := s.store.PurgeWritingTestPost(ctx, user, slug); err != nil {
		return err
	}
	if err := s.cancel(ctx, fences); err != nil {
		return err
	}
	pending, err := s.store.ListCancelledUnsettledTestExecutions(ctx)
	if err != nil {
		return err
	}
	owned := make([]experiment.TestExecutionFence, 0, len(pending))
	for _, f := range pending {
		if f.UserID == user {
			owned = append(owned, f)
		}
	}
	if err := s.cancel(ctx, owned); err != nil {
		return err
	}
	return s.legacy.PurgePost(ctx, user, slug)
}

func (s *WritingTestLifecycle) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	current, err := s.store.PurgeExpiredWritingTests(ctx, before)
	if err != nil {
		return current, err
	}
	legacy, err := s.legacy.PurgeExpired(ctx, before)
	if err != nil {
		return current + legacy, err
	}
	pending, err := s.store.ListCancelledUnsettledTestExecutions(ctx)
	if err != nil {
		return current + legacy, err
	}
	if err := s.cancel(ctx, pending); err != nil {
		return current + legacy, err
	}
	if err := s.jobs.ReconcileCancelledSettlements(ctx, pending); err != nil {
		return current + legacy, err
	}
	return current + legacy, s.jobs.ReconcileTerminalSettlements(ctx)
}

// Recover follows queue interruption, deferred-admission and hold recovery and
// precedes all worker/HTTP admission. It never reconstructs private payloads.
func (s *WritingTestLifecycle) Recover(ctx context.Context) error {
	if err := s.store.RecoverInterruptedTests(ctx); err != nil {
		return err
	}
	if err := s.cancelPending(ctx); err != nil {
		return err
	}
	return s.jobs.RecoverSettlements(ctx)
}
func (s *WritingTestLifecycle) cancelPending(ctx context.Context) error {
	fences, err := s.store.ListCancelledUnsettledTestExecutions(ctx)
	if err != nil {
		return err
	}
	return s.cancel(ctx, fences)
}
func (s *WritingTestLifecycle) cancel(ctx context.Context, fences []experiment.TestExecutionFence) error {
	var errs []error
	seen := map[string]bool{}
	for _, f := range fences {
		if f.JobID == "" || seen[f.JobID] {
			continue
		}
		seen[f.JobID] = true
		if err := s.jobs.CancelWritingTestJob(ctx, f.UserID, f.JobID); err != nil && !errors.Is(err, job.ErrNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
