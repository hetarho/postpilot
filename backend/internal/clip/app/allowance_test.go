package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

// approvedCalls is an approval for `count` observations and one writing line, priced the
// way a quote prices them.
func approvedCalls(count int) Reservation {
	return Reservation{ApprovedMaxCredits: 100, Calls: []Call{
		{Policy: observePolicy(), Count: count},
		{Policy: writePolicy(), Count: 1},
	}}
}

func plannedCalls(observations int) []job.PlannedCall {
	return []job.PlannedCall{
		{Ref: observePolicy().Ref.String(), Count: observations, CompletionTokens: observePolicy().CompletionTokens},
		{Ref: writePolicy().Ref.String(), Count: 1, CompletionTokens: writePolicy().CompletionTokens},
	}
}

func reserved(t *testing.T, observations int) (context.Context, *fakeReserver) {
	t.Helper()
	reserver := &fakeReserver{}
	jobs := NewJobs(&fakeQueue{summary: reservableJob()}, reserver)
	ctx, err := jobs.Reserve(context.Background(), "alice", "job", plannedCalls(observations), approvedCalls(observations))
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	return ctx, reserver
}

func TestReserveRefusesEveryJobThatIsNotTheApprovedOne(t *testing.T) {
	reserver := &fakeReserver{}
	for name, mutate := range map[string]func(*job.JobSummary){
		"foreign user":     func(j *job.JobSummary) { j.UserID = "bob" },
		"render kind":      func(j *job.JobSummary) { j.Kind = clip.JobKindRender },
		"queued":           func(j *job.JobSummary) { j.Status = job.StatusQueued },
		"past prepare":     func(j *job.JobSummary) { j.Stage = "analyze" },
		"other model":      func(j *job.JobSummary) { j.ObserveModel = "p/other" },
		"policy mismatch":  func(j *job.JobSummary) { j.CancellationPolicyVersion = 1 },
		"cancel requested": func(j *job.JobSummary) { now := j.CreatedAt; j.CancelRequestedAt = &now },
	} {
		summary := reservableJob()
		mutate(summary)
		jobs := NewJobs(&fakeQueue{summary: summary}, reserver)
		if _, err := jobs.Reserve(context.Background(), "alice", "job", plannedCalls(3), approvedCalls(3)); !errors.Is(err, clip.ErrCreditAllowance) {
			t.Fatalf("%s: err = %v, want no allowance", name, err)
		}
	}
	if len(reserver.held) != 0 {
		t.Fatal("a refused reservation reached the ledger", reserver.held)
	}
}

func TestReserveRefusesAnApprovalThatDoesNotMatchThePlannedCalls(t *testing.T) {
	jobs := NewJobs(&fakeQueue{summary: reservableJob()}, &fakeReserver{})
	widened := approvedCalls(3)
	widened.Calls[0].Count = 4
	for name, args := range map[string]struct {
		calls    []job.PlannedCall
		approval Reservation
	}{
		"approval wider than the plan": {plannedCalls(3), widened},
		"no planned call at all":       {nil, approvedCalls(3)},
		"one approved line":            {plannedCalls(3), Reservation{ApprovedMaxCredits: 100, Calls: []Call{{Policy: observePolicy(), Count: 3}}}},
		"negative ceiling":             {plannedCalls(3), Reservation{ApprovedMaxCredits: -1, Calls: approvedCalls(3).Calls}},
	} {
		if _, err := jobs.Reserve(context.Background(), "alice", "job", args.calls, args.approval); !errors.Is(err, clip.ErrCreditAllowance) {
			t.Fatalf("%s: err = %v, want no allowance", name, err)
		}
	}
}

func TestARefusedHoldReturnsNoUsableAllowance(t *testing.T) {
	refusal := errors.New("not enough credits")
	jobs := NewJobs(&fakeQueue{summary: reservableJob()}, &fakeReserver{refuse: refusal})
	ctx, err := jobs.Reserve(context.Background(), "alice", "job", plannedCalls(1), approvedCalls(1))
	if ctx != nil || !errors.Is(err, refusal) {
		t.Fatal(ctx, err)
	}
}

func TestConsumeSpendsExactlyTheAllowanceAndNoMore(t *testing.T) {
	ctx, _ := reserved(t, 3)
	observe, write := observePolicy(), writePolicy()
	if _, err := ConsumePolicy(ctx, "alice", "job", observe.Ref.String(), observe.CompletionTokens, "write"); !errors.Is(err, clip.ErrCreditAllowance) {
		t.Fatal("a call consumed another stage's allowance", err)
	}
	for name, bad := range map[string]struct {
		user, id, ref string
		budget        int
	}{
		"foreign user":  {"bob", "job", observe.Ref.String(), observe.CompletionTokens},
		"other job":     {"alice", "other", observe.Ref.String(), observe.CompletionTokens},
		"other model":   {"alice", "job", "p/other", observe.CompletionTokens},
		"other budget":  {"alice", "job", observe.Ref.String(), observe.CompletionTokens + 1},
		"unknown stage": {"alice", "job", write.Ref.String(), observe.CompletionTokens},
	} {
		if err := ConsumeCall(ctx, bad.user, bad.id, bad.ref, bad.budget); !errors.Is(err, clip.ErrCreditAllowance) {
			t.Fatalf("%s: err = %v, want no allowance", name, err)
		}
	}
	// Failed calls consume a slot too, so twenty racing calls may not spend four.
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ConsumeCall(ctx, "alice", "job", observe.Ref.String(), observe.CompletionTokens) == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 3 {
		t.Fatalf("the allowance spent %d calls, want 3", successes.Load())
	}
	if err := ConsumeCall(ctx, "alice", "job", write.Ref.String(), write.CompletionTokens); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeCall(ctx, "alice", "job", write.Ref.String(), write.CompletionTokens); !errors.Is(err, clip.ErrCreditAllowance) {
		t.Fatal("an unplanned repair call was admitted", err)
	}
	if err := ConsumeCall(context.Background(), "alice", "job", write.Ref.String(), write.CompletionTokens); !errors.Is(err, clip.ErrCreditAllowance) {
		t.Fatal("a call outside the reservation was admitted", err)
	}
}

// One correction per response retry, and not one more: the approval prices them.
func TestCorrectionAllowanceStopsAtTheApprovedRetries(t *testing.T) {
	approval := approvedCalls(4)
	calls := plannedCalls(4)
	for i := range approval.Calls {
		approval.Calls[i].Policy.ResponseRetries = 3
		approval.Calls[i].Count = 4
	}
	calls[1].Count = 4
	reserver := &fakeReserver{}
	jobs := NewJobs(&fakeQueue{summary: reservableJob()}, reserver)
	ctx, err := jobs.Reserve(context.Background(), "alice", "job", calls, approval)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range approval.Calls {
		for range 4 {
			policy, err := ConsumePolicy(ctx, "alice", "job", c.Policy.Ref.String(), c.Policy.CompletionTokens, c.Policy.Stage)
			if err != nil || policy != c.Policy {
				t.Fatal("lost the frozen correction allowance", err)
			}
		}
		if _, err := ConsumePolicy(ctx, "alice", "job", c.Policy.Ref.String(), c.Policy.CompletionTokens, c.Policy.Stage); !errors.Is(err, clip.ErrCreditAllowance) {
			t.Fatal("a fifth call was admitted", err)
		}
	}
}

// The reserved context carries the approval to the ledger exactly as approved.
func TestTheHoldCarriesTheApprovedPricing(t *testing.T) {
	_, reserver := reserved(t, 2)
	if len(reserver.held) != 1 {
		t.Fatal(reserver.held)
	}
	held := reserver.held[0]
	if held.UserID != "alice" || held.JobID != "job" || held.Kind != clip.JobKindGenerate {
		t.Fatal(held)
	}
	if held.Reservation.ApprovedMaxCredits != 100 || len(held.Reservation.Calls) != 2 || held.Reservation.Calls[0].Count != 2 {
		t.Fatal("the ledger lost the approved ceiling", held.Reservation)
	}
	if held.Reservation.Calls[0].Policy.InputUSDPerMillion != observePolicy().InputUSDPerMillion {
		t.Fatal("the ledger lost the frozen price")
	}
}

// A dispatch that lost the race with the owner's cancellation admits no call.
func TestARefusedAuthorizationStopsTheCall(t *testing.T) {
	reserver := &fakeReserver{refused: job.ErrDispatchRefused}
	jobs := NewJobs(&fakeQueue{summary: reservableJob()}, reserver)
	ctx, err := jobs.Reserve(context.Background(), "alice", "job", plannedCalls(1), approvedCalls(1))
	if err != nil {
		t.Fatal(err)
	}
	if err := ConsumeCall(ctx, "alice", "job", observePolicy().Ref.String(), observePolicy().CompletionTokens); !errors.Is(err, clip.ErrCreditAllowance) {
		t.Fatal("a cancelled job still admitted a call", err)
	}
}

// Work with no reserver at all cannot reserve: a queue without the guard is a queue that
// cannot spend, which is the mode the queue's own tests run in.
func TestWithoutAGuardNothingReserves(t *testing.T) {
	jobs := NewJobs(&fakeQueue{summary: reservableJob()}, nil)
	if _, err := jobs.Reserve(context.Background(), "alice", "job", plannedCalls(1), approvedCalls(1)); !errors.Is(err, clip.ErrCreditAllowance) {
		t.Fatal(err)
	}
}

var _ = llm.CallPolicy{}
