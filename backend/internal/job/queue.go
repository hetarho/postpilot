package job

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// releaseTimeout bounds the one compensating delete. It is a single indexed statement on the
// writer, so anything longer than this is a stuck database, not a slow query.
const releaseTimeout = 5 * time.Second

// Enqueue persists queued work and only then wakes the worker. It never runs the
// handler in the caller's request.
func (q *Queue) Enqueue(ctx context.Context, input NewJob) (string, error) {
	// The queue honours one cancellation policy version besides "none"; which kinds may
	// carry it is the enqueueing context's rule, checked before it gets here.
	if input.CancellationPolicyVersion < 0 || input.CancellationPolicyVersion > 1 {
		return "", ErrInvalidTarget
	}
	if input.Kind == "" || input.UserID == "" {
		return "", fmt.Errorf("enqueue job: kind and user are required")
	}
	for _, s := range input.Subjects {
		if !s.valid() {
			return "", ErrInvalidTarget
		}
	}
	// Work that spends nothing may not also defer a hold it will never take.
	if input.NonMetered && input.DeferHold {
		return "", ErrInvalidTarget
	}
	if (input.Kind == KindGenerate || input.Kind == KindRevise) && input.TargetLanguage == "" {
		return "", fmt.Errorf("enqueue job: target language is required for %s", input.Kind)
	}
	if input.TargetLanguage != "" && input.TargetLanguage != "ko" && input.TargetLanguage != "en" {
		return "", fmt.Errorf("enqueue job: unsupported target language %q", input.TargetLanguage)
	}

	active, err := q.activeForInput(ctx, input)
	if err != nil {
		return "", fmt.Errorf("check active job: %w", err)
	}
	if active != nil {
		return "", &ErrAlreadyInProgress{ActiveID: active.ID}
	}

	now := q.now()
	found := Job{
		CancellationPolicyVersion: input.CancellationPolicyVersion,
		ID:                        q.newID(), Kind: input.Kind, UserID: input.UserID, Subjects: cloneSubjects(input.Subjects),
		Status: StatusQueued, ObserveModel: input.ObserveModel, WriteModel: input.WriteModel,
		TargetLanguage: input.TargetLanguage,
		Payload:        append([]byte(nil), input.Payload...), CreatedAt: now, UpdatedAt: now,
	}

	// The hold precedes the insert so a refused start leaves no job row at all. The error
	// is returned unwrapped: the credit refusal it carries is matched by type at every rpc
	// edge above, and wrapping it here would say nothing a caller needs.
	// Work that defers its hold reserves its credits when it knows what it will spend
	// them on — after its owner approves — so nothing is held for it here.
	if q.admitter != nil && !input.DeferHold && !input.NonMetered {
		if err := q.admitter.Hold(ctx, Start{
			UserID: input.UserID, Kind: input.Kind, JobID: found.ID, Calls: input.plannedCalls(),
		}); err != nil {
			return "", err
		}
	}

	if err := q.store.Insert(ctx, found); err != nil {
		if !input.NonMetered {
			q.releaseAdmission(ctx, found.ID)
		}
		if errors.Is(err, ErrActiveConflict) {
			active, lookupErr := q.activeForInput(ctx, input)
			if lookupErr == nil && active != nil {
				return "", &ErrAlreadyInProgress{ActiveID: active.ID}
			}
		}
		return "", fmt.Errorf("insert job: %w", err)
	}

	select {
	case q.wake <- struct{}{}:
	default:
	}
	return found.ID, nil
}

// releaseAdmission returns a hold whose job row was never created.
//
// It deliberately drops the caller's cancellation: the most likely reason the insert failed
// is that the request went away, and releasing on that same dead context would leave the
// account charged for a start it never got.
func (q *Queue) releaseAdmission(ctx context.Context, jobID string) {
	if q.admitter == nil {
		return
	}
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()
	q.admitter.Release(releaseCtx, jobID)
}

// activeForInput runs the guards the caller stated, in order, and stops at the first one
// that finds active work. Which subject serializes which kind is the enqueueing context's
// rule — voice-owned work states both its post guard and its (voice, kind) guard — so the
// queue only walks the list. No guard at all means the unattached default.
func (q *Queue) activeForInput(ctx context.Context, input NewJob) (*Job, error) {
	if len(input.Guards) == 0 {
		return q.store.ActiveUnattached(ctx, input.UserID, input.Kind)
	}
	for _, guard := range input.Guards {
		if !guard.Subject.valid() {
			return nil, ErrInvalidTarget
		}
		active, err := q.store.ActiveFor(ctx, guard.Subject, guard.Filter)
		if err != nil || active != nil {
			return active, err
		}
	}
	return nil, nil
}
