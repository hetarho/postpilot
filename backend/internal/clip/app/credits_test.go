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
				if !errors.Is(err, job.ErrCreditAllowance) {
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
	reserved []job.PlannedCall
	approval []job.ClipReservation
}

func (q *fakeQueue) ReserveClip(ctx context.Context, _, _ string, calls []job.PlannedCall, approval ...job.ClipReservation) (context.Context, error) {
	q.reserved, q.approval = calls, approval
	return ctx, nil
}

func TestReserveApprovedSkipsTheQueueWhenNothingIsLeftToCall(t *testing.T) {
	q := &fakeQueue{}
	jobs := NewJobs(q)
	// A resumed generation that already holds every observation and both
	// writing results has nothing left to price.
	resumed := approvedPricing(0)
	resumed.SkipFlow, resumed.SkipNarration = true, true
	resumed.MaxCredits, _ = QuoteCredits(resumed, 0, DefaultQuoteRetries)
	if !resumed.Valid() {
		t.Fatal("the resumed quote must be a valid render-only pricing")
	}
	approval := clip.GenerationApproval{QuoteID: "quote", MaxCredits: resumed.MaxCredits, Pricing: resumed}
	if _, err := jobs.ReserveApproved(context.Background(), "alice", "job", approval, 0); err != nil || q.reserved != nil {
		t.Fatal("a continuation with nothing left to call must not reserve", q.reserved, err)
	}
	fresh := approvedPricing(1)
	approval = clip.GenerationApproval{QuoteID: "quote", MaxCredits: fresh.MaxCredits, Pricing: fresh}
	if _, err := jobs.ReserveApproved(context.Background(), "alice", "job", approval, 1); err != nil || len(q.reserved) != 2 || len(q.approval) != 1 {
		t.Fatal(q.reserved, q.approval, err)
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
