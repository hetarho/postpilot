package job

import (
	"context"
	"errors"
	"math"
	"sync"
)

const KindGenerateClip = "generate_clip"
const KindRenderClip = "render_clip"

var ErrCreditAllowance = errors.New("clip call has no reserved credit allowance")

// ClipStore is the extension used only by the deferred-admission clip queue path.
type ClipStore interface {
	LatestForClip(context.Context, string, string) (*Job, error)
	ActiveForClip(context.Context, string, string) (*Job, error)
	ActivateClip(context.Context, string, string) (bool, error)
	SweepUnactivatedClips(context.Context, Failure) (int64, error)
}

func (q *Queue) LatestForClip(ctx context.Context, user, id string) (*JobSummary, error) {
	s, ok := q.store.(ClipStore)
	if !ok {
		return nil, errors.New("clip job store unavailable")
	}
	j, err := s.LatestForClip(ctx, user, id)
	if err != nil || j == nil {
		return nil, err
	}
	return summarize(*j), nil
}

func (q *Queue) ActiveForClip(ctx context.Context, user, id string) (*JobSummary, error) {
	s, ok := q.store.(ClipStore)
	if !ok {
		return nil, errors.New("clip job store unavailable")
	}
	j, err := s.ActiveForClip(ctx, user, id)
	if j == nil || err != nil {
		return nil, err
	}
	return summarize(*j), nil
}
func (q *Queue) ActivateClip(ctx context.Context, user, id string) error {
	s, ok := q.store.(ClipStore)
	if !ok {
		return errors.New("clip job store unavailable")
	}
	yes, err := s.ActivateClip(ctx, user, id)
	if err != nil {
		return err
	}
	if !yes {
		return ErrNotFound
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return nil
}

// Boot only, before any worker starts. The unactivated row has made no model call.
func (q *Queue) SweepUnactivatedClips(ctx context.Context) (int64, error) {
	s, ok := q.store.(ClipStore)
	if !ok {
		return 0, errors.New("clip job store unavailable")
	}
	return s.SweepUnactivatedClips(ctx, interruptedFailure)
}

type allowanceKey struct{}
type callKey struct {
	ref    string
	budget int
}
type allowance struct {
	mu        sync.Mutex
	user, job string
	remaining map[callKey]int
}

// ReserveClip is deliberately worker-only: a queued/foreign/different-kind job cannot
// acquire an allowance. A failed Hold returns no usable context. The ledger's unique
// job key prevents a second successful reservation, including after a crash.
func (q *Queue) ReserveClip(ctx context.Context, user, id string, calls []PlannedCall) (context.Context, error) {
	j, err := q.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.UserID != user || j.Kind != KindGenerateClip || j.Status != StatusRunning || j.Stage != "prepare" || q.admitter == nil {
		return nil, ErrCreditAllowance
	}
	remaining := map[callKey]int{}
	if len(calls) == 0 {
		return nil, ErrCreditAllowance
	}
	for _, c := range calls {
		if c.Ref == "" || (c.Ref != j.ObserveModel && c.Ref != j.WriteModel) || c.Count <= 0 || c.CompletionTokens <= 0 {
			return nil, ErrCreditAllowance
		}
		key := callKey{c.Ref, c.CompletionTokens}
		if c.Count > math.MaxInt-remaining[key] {
			return nil, ErrCreditAllowance
		}
		remaining[key] += c.Count
	}
	if err := q.admitter.Hold(ctx, Start{UserID: user, Kind: j.Kind, JobID: id, Calls: normalizePlannedCalls(calls)}); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, allowanceKey{}, &allowance{user: user, job: id, remaining: remaining}), nil
}

// ConsumeClipCall runs at the metered LLM boundary, before any provider invocation.
// Failed calls consume a slot too: an unplanned retry is still an extra paid call.
func ConsumeClipCall(ctx context.Context, user, id, ref string, budget int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a, ok := ctx.Value(allowanceKey{}).(*allowance)
	if !ok || a.user != user || a.job != id {
		return ErrCreditAllowance
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	key := callKey{ref, budget}
	if a.remaining[key] <= 0 {
		return ErrCreditAllowance
	}
	a.remaining[key]--
	return nil
}
