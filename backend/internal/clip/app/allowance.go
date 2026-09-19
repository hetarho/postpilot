package app

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

// Call is one priced model call the reservation approved, and how many times it may run.
type Call struct {
	Policy llm.CallPolicy
	Count  int
}

// Reservation is what an approved quote allows: a credit ceiling, the cancellation policy
// the owner approved under, and the two priced lines (observation, writing) it covers.
type Reservation struct {
	CancellationPolicyVersion int
	ApprovedMaxCredits        int
	Calls                     []Call
}

// Hold is the credit hold a charged clip job takes before its first model call. The
// ledger adapter at the composition root translates it; the queue never carries it.
type Hold struct {
	UserID, Kind, JobID string
	Calls               []job.PlannedCall
	Reservation         Reservation
}

// The completion budgets and delivery shapes a clip reservation is allowed to name. They
// are the assembly contract's, checked here so an approval cannot widen what it pays for.
const (
	observeCompletionTokens = 8192
	writeCompletionTokens   = 32768
	// One clip makes TWO writing calls on the same model at the same budget — the footage
	// flow, then the narration over it — so the writing line carries both, each with its
	// own response corrections.
	writeCallsPerClip = 2
)

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
	authorize   func(context.Context, string, string) error
}

// Reserve holds the credits an approved quote allows and returns the context the metered
// boundary consumes them from.
//
// It is deliberately worker-only: a queued, foreign or different-kind job cannot acquire
// an allowance. A failed hold returns no usable context, and the ledger's unique job key
// prevents a second successful reservation, including after a crash.
func (a Jobs) Reserve(ctx context.Context, user, id string, calls []job.PlannedCall, approval Reservation) (context.Context, error) {
	if approval.ApprovedMaxCredits < 0 || len(approval.Calls) != 2 || len(calls) == 0 || a.guard == nil {
		return nil, clip.ErrCreditAllowance
	}
	j, err := a.queue.Get(ctx, id, user)
	if err != nil {
		return nil, err
	}
	if j == nil || j.UserID != user || j.Kind == clip.JobKindRender || !clip.IsJobKind(j.Kind) || j.Status != job.StatusRunning ||
		j.Stage != "prepare" || j.CancelRequestedAt != nil || j.CancellationPolicyVersion != approval.CancellationPolicyVersion {
		return nil, clip.ErrCreditAllowance
	}
	remaining := map[callKey]int{}
	for _, c := range calls {
		if c.Ref == "" || (c.Ref != j.ObserveModel && c.Ref != j.WriteModel) || c.Count <= 0 || c.CompletionTokens <= 0 {
			return nil, clip.ErrCreditAllowance
		}
		key := callKey{c.Ref, c.CompletionTokens}
		if c.Count > math.MaxInt-remaining[key] {
			return nil, clip.ErrCreditAllowance
		}
		remaining[key] += c.Count
	}
	policies, err := approvedPolicies(approval, *j, remaining)
	if err != nil {
		return nil, err
	}
	approved := approval
	approved.Calls = append([]Call(nil), approved.Calls...)
	hold := Hold{UserID: user, Kind: j.Kind, JobID: id, Calls: calls, Reservation: approved}
	if err := a.guard.Reserve(ctx, hold); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, allowanceKey{}, &allowance{
		user: user, job: id, remaining: remaining, policies: policies,
		approvedMax: approved.ApprovedMaxCredits, authorize: a.guard.Authorize,
	}), nil
}

// approvedPolicies checks each approved line against the job's frozen model choices and
// the assembly contract's budgets, and returns the policies the allowance will hand out.
func approvedPolicies(approval Reservation, j job.JobSummary, remaining map[callKey]int) (map[callKey]llm.CallPolicy, error) {
	policies := map[callKey]llm.CallPolicy{}
	for i, c := range approval.Calls {
		p := c.Policy
		if !p.Valid() || !p.Pricing.Valid() || c.Count < 0 || c.Count > 49*(1+p.ResponseRetries) {
			return nil, clip.ErrCreditAllowance
		}
		if i == 0 && (p.Stage != "observe" || p.Ref.String() != j.ObserveModel || p.CompletionTokens != observeCompletionTokens || p.Pricing.Delivery != llm.ExecutionInlineStatic) {
			return nil, clip.ErrCreditAllowance
		}
		if i == 1 && (p.Stage != "write" || p.Ref.String() != j.WriteModel || c.Count > writeCallsPerClip*(1+p.ResponseRetries) || p.CompletionTokens != writeCompletionTokens || p.Pricing.Delivery != llm.ExecutionTextOnly) {
			return nil, clip.ErrCreditAllowance
		}
		if c.Count == 0 {
			continue
		}
		key := callKey{p.Ref.String(), p.CompletionTokens}
		if remaining[key] != c.Count {
			return nil, clip.ErrCreditAllowance
		}
		policies[key] = p
	}
	if len(policies) != len(remaining) {
		return nil, clip.ErrCreditAllowance
	}
	return policies, nil
}

// ConsumeCall runs at the metered LLM boundary, before any provider invocation. Failed
// calls consume a slot too: an unplanned retry is still an extra paid call.
func ConsumeCall(ctx context.Context, user, id, ref string, budget int) error {
	_, err := consumePolicy(ctx, user, id, ref, budget, "")
	return err
}

// ConsumePolicy is the same consumption for a call that names its stage, returning the
// frozen price the ledger records the call at.
func ConsumePolicy(ctx context.Context, user, id, ref string, budget int, stage string) (llm.CallPolicy, error) {
	if stage == "" {
		return llm.CallPolicy{}, clip.ErrCreditAllowance
	}
	return consumePolicy(ctx, user, id, ref, budget, stage)
}

func consumePolicy(ctx context.Context, user, id, ref string, budget int, stage string) (llm.CallPolicy, error) {
	if err := ctx.Err(); err != nil {
		return llm.CallPolicy{}, err
	}
	a, ok := ctx.Value(allowanceKey{}).(*allowance)
	if !ok || a.user != user || a.job != id {
		return llm.CallPolicy{}, clip.ErrCreditAllowance
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	key := callKey{ref, budget}
	p, ok := a.policies[key]
	if a.remaining[key] <= 0 || !ok || !p.Valid() || (stage != "" && p.Stage != stage) {
		return llm.CallPolicy{}, clip.ErrCreditAllowance
	}
	if a.authorize != nil {
		if err := a.authorize(ctx, user, id); err != nil {
			// A dispatch that lost the race with the owner's cancellation is refused work
			// like any other unreserved call, and is reported as such.
			if errors.Is(err, job.ErrDispatchRefused) {
				return llm.CallPolicy{}, clip.ErrCreditAllowance
			}
			return llm.CallPolicy{}, err
		}
	}
	a.remaining[key]--
	return p, nil
}
