package app

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

func TestQuoteCreditsPricesRetriesAndBothWritingCallsAsOneLine(t *testing.T) {
	base := clip.GenerationPricing{Observe: observePolicy(), Plan: writePolicy(), Narration: writePolicy(), ObservationCalls: 2}
	one, err := QuoteCredits(base, 2, 0)
	if err != nil || one <= 0 {
		t.Fatal(one, err)
	}
	withRetries, err := QuoteCredits(base, 2, DefaultQuoteRetries)
	if err != nil || withRetries <= one {
		t.Fatal("retries must raise the ceiling", one, withRetries, err)
	}
	renderOnly := base
	renderOnly.SkipFlow, renderOnly.SkipNarration = true, true
	if renderOnly.PlanCalls() != 0 {
		t.Fatal("render-only quote still counts writing calls", renderOnly.PlanCalls())
	}
	cheaper, err := QuoteCredits(renderOnly, 2, 0)
	if err != nil || cheaper >= one {
		t.Fatal("skipping both writing calls must cost less", cheaper, one, err)
	}
}

func TestPlanReservationEnforcesTheApprovalRule(t *testing.T) {
	pricing := approvedPricing(3)
	ok := clip.GenerationApproval{QuoteID: "quote", MaxCredits: pricing.MaxCredits, Pricing: pricing}
	cases := []struct {
		name     string
		approval clip.GenerationApproval
		chunks   int
		wantErr  bool
		calls    int
	}{
		{"valid", ok, 3, false, 2},
		{"fewer chunks than quoted", ok, 1, false, 2},
		{"no observation left", ok, 0, false, 1},
		{"more chunks than quoted", ok, 4, true, 0},
		{"negative chunks", ok, -1, true, 0},
		{"missing quote id", clip.GenerationApproval{MaxCredits: pricing.MaxCredits, Pricing: pricing}, 3, true, 0},
		{"ceiling disagrees with quote", clip.GenerationApproval{QuoteID: "quote", MaxCredits: pricing.MaxCredits + 1, Pricing: pricing}, 3, true, 0},
		{"invalid pricing", clip.GenerationApproval{QuoteID: "quote", Pricing: clip.GenerationPricing{}}, 0, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			calls, reservation, err := PlanReservation(c.approval, c.chunks)
			if c.wantErr {
				if !errors.Is(err, clip.ErrCreditAllowance) {
					t.Fatal(err)
				}
				return
			}
			if err != nil || len(calls) != c.calls {
				t.Fatal(calls, err)
			}
			if reservation.ApprovedMaxCredits != pricing.MaxCredits || reservation.CancellationPolicyVersion != pricing.CancellationPolicyVersion || len(reservation.Calls) != 2 {
				t.Fatal(reservation)
			}
			if c.chunks > 0 && (calls[0].Ref != "p/o" || calls[0].Count != pricing.ObserveCalls(c.chunks)) {
				t.Fatal(calls[0])
			}
		})
	}
}

type fakeQueue struct {
	Queue
	summary *job.JobSummary
}

func (q *fakeQueue) Get(context.Context, string, string) (*job.JobSummary, error) {
	return q.summary, nil
}

// reservableJob is the job an approved reservation may be taken against: running, in its
// prepare stage, on the models the approval prices.
func reservableJob() *job.JobSummary {
	return &job.JobSummary{
		ID: "job", Kind: clip.JobKindGenerate, UserID: "alice", Status: job.StatusRunning, Stage: "prepare",
		ObserveModel: observePolicy().Ref.String(), WriteModel: writePolicy().Ref.String(),
	}
}

// fakeReserver records the hold the allowance would take and admits it.
type fakeReserver struct {
	held    []Hold
	refuse  error
	refused error
}

func (r *fakeReserver) Reserve(_ context.Context, hold Hold) error {
	if r.refuse != nil {
		return r.refuse
	}
	r.held = append(r.held, hold)
	return nil
}

func (r *fakeReserver) Authorize(context.Context, string, string) error { return r.refused }

func TestReserveApprovedSkipsTheQueueWhenNothingIsLeftToCall(t *testing.T) {
	q := &fakeQueue{summary: reservableJob()}
	reserver := &fakeReserver{}
	jobs := NewJobs(q, reserver)
	// A resumed generation that already holds every observation and both
	// writing results has nothing left to price.
	resumed := approvedPricing(0)
	resumed.SkipFlow, resumed.SkipNarration = true, true
	resumed.MaxCredits, _ = QuoteCredits(resumed, 0, DefaultQuoteRetries)
	if !resumed.Valid() {
		t.Fatal("the resumed quote must be a valid render-only pricing")
	}
	approval := clip.GenerationApproval{QuoteID: "quote", MaxCredits: resumed.MaxCredits, Pricing: resumed}
	if _, err := jobs.ReserveApproved(context.Background(), "alice", "job", approval, 0); err != nil || len(reserver.held) != 0 {
		t.Fatal("a continuation with nothing left to call must not reserve", reserver.held, err)
	}
	fresh := approvedPricing(1)
	approval = clip.GenerationApproval{QuoteID: "quote", MaxCredits: fresh.MaxCredits, Pricing: fresh}
	if _, err := jobs.ReserveApproved(context.Background(), "alice", "job", approval, 1); err != nil || len(reserver.held) != 1 || len(reserver.held[0].Calls) != 2 {
		t.Fatal(reserver.held, err)
	}
}

type fakeFreezer struct {
	calls  []string
	err    error
	models []llm.ModelInfo
}

func (f *fakeFreezer) FreezeExecution(_ context.Context, ref llm.ModelRef, stage string, budget int, _ llm.ReasoningEffort, delivery llm.ExecutionDelivery) (llm.CallPolicy, error) {
	f.calls = append(f.calls, stage)
	if f.err != nil {
		return llm.CallPolicy{}, f.err
	}
	p := observePolicy()
	if stage == "write" {
		p = writePolicy()
	}
	p.Ref, p.CompletionTokens = ref, budget
	p.Pricing.Delivery = delivery
	return p, nil
}
func (f *fakeFreezer) Models() []llm.ModelInfo { return f.models }

func TestPricingFreezesThreeCallsAndMapsRefusals(t *testing.T) {
	freezer := &fakeFreezer{}
	pricing := NewPricing(freezer, Budgets{ObserveCompletionTokens: 8192, FlowCompletionTokens: 4096, NarrationCompletionTokens: 2048})
	got, err := pricing.Freeze(context.Background(), llm.ModelRef{ProviderID: "p", ModelID: "o"}, llm.ModelRef{ProviderID: "p", ModelID: "w"}, 2)
	if err != nil || len(freezer.calls) != 3 || got.MaxCredits <= 0 || got.Observe.ResponseRetries != DefaultQuoteRetries || got.Plan.CompletionTokens != 4096 || got.Narration.CompletionTokens != 2048 {
		t.Fatal(got, freezer.calls, err)
	}
	freezer.err = errors.New("provider down")
	if _, err := pricing.Freeze(context.Background(), llm.ModelRef{}, llm.ModelRef{}, 1); !errors.Is(err, clip.ErrPricingUnavailable) {
		t.Fatal("a generic freeze failure is the generic pricing refusal", err)
	}
}
