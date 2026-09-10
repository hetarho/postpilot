package job

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/llm"
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

// LatestClipSnapshot is an internal owner-scoped read for approval recovery. The
// durable payload is never added to the public job summary or RPC projection.
func (q *Queue) LatestClipSnapshot(ctx context.Context, user, id string) (*Job, error) {
	s, ok := q.store.(ClipStore)
	if !ok {
		return nil, errors.New("clip job store unavailable")
	}
	j, err := s.LatestForClip(ctx, user, id)
	if err != nil || j == nil {
		return nil, err
	}
	if j.UserID != user || j.ClipProjectID != id {
		return nil, ErrNotFound
	}
	return j, nil
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
	mu          sync.Mutex
	user, job   string
	remaining   map[callKey]int
	policies    map[callKey]llm.CallPolicy
	approvedMax int
}

// ReserveClip is deliberately worker-only: a queued/foreign/different-kind job cannot
// acquire an allowance. A failed Hold returns no usable context. The ledger's unique
// job key prevents a second successful reservation, including after a crash.
func (q *Queue) ReserveClip(ctx context.Context, user, id string, calls []PlannedCall, approval ...ClipReservation) (context.Context, error) {
	if len(approval) != 1 || approval[0].ApprovedMaxCredits < 0 || len(approval[0].Calls) != 2 {
		return nil, ErrCreditAllowance
	}
	j, err := q.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.UserID != user || j.Kind != KindGenerateClip || j.Status != StatusRunning || j.Stage != "prepare" || q.admitter == nil {
		return nil, ErrCreditAllowance
	}
	remaining := map[callKey]int{}
	policies := map[callKey]llm.CallPolicy{}
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
	for i, c := range approval[0].Calls {
		p := c.Policy
		if !p.Valid() || c.Count <= 0 || c.Count > 49 {
			return nil, ErrCreditAllowance
		}
		if i == 0 && (p.Stage != "observe" || p.Ref.String() != j.ObserveModel || p.CompletionTokens != 8192) {
			return nil, ErrCreditAllowance
		}
		if i == 1 && (p.Stage != "write" || p.Ref.String() != j.WriteModel || c.Count != 1 || p.CompletionTokens != 32768) {
			return nil, ErrCreditAllowance
		}
		key := callKey{p.Ref.String(), p.CompletionTokens}
		if remaining[key] != c.Count {
			return nil, ErrCreditAllowance
		}
		policies[key] = p
	}
	if len(policies) != len(remaining) {
		return nil, ErrCreditAllowance
	}
	approved := approval[0]
	approved.Calls = append([]ClipCall(nil), approved.Calls...)
	if err := q.admitter.Hold(ctx, Start{UserID: user, Kind: j.Kind, JobID: id, Calls: normalizePlannedCalls(calls), Clip: &approved}); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, allowanceKey{}, &allowance{user: user, job: id, remaining: remaining, policies: policies, approvedMax: approved.ApprovedMaxCredits}), nil
}

// ConsumeClipCall runs at the metered LLM boundary, before any provider invocation.
// Failed calls consume a slot too: an unplanned retry is still an extra paid call.
func ConsumeClipCall(ctx context.Context, user, id, ref string, budget int) error {
	_, err := consumeClipPolicy(ctx, user, id, ref, budget, "")
	return err
}

func ConsumeClipPolicy(ctx context.Context, user, id, ref string, budget int, stage string) (llm.CallPolicy, error) {
	if stage == "" {
		return llm.CallPolicy{}, ErrCreditAllowance
	}
	return consumeClipPolicy(ctx, user, id, ref, budget, stage)
}

func consumeClipPolicy(ctx context.Context, user, id, ref string, budget int, stage string) (llm.CallPolicy, error) {
	if err := ctx.Err(); err != nil {
		return llm.CallPolicy{}, err
	}
	a, ok := ctx.Value(allowanceKey{}).(*allowance)
	if !ok || a.user != user || a.job != id {
		return llm.CallPolicy{}, ErrCreditAllowance
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	key := callKey{ref, budget}
	p, ok := a.policies[key]
	if a.remaining[key] <= 0 || !ok || !p.Valid() || (stage != "" && p.Stage != stage) {
		return llm.CallPolicy{}, ErrCreditAllowance
	}
	a.remaining[key]--
	return p, nil
}
