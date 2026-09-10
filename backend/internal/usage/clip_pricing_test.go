package usage

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

func approvedTestClip() *ClipReservation {
	return &ClipReservation{ApprovedMaxCredits: 100, Calls: []PricedCall{
		{Policy: llm.CallPolicy{Ref: cheapRef, Stage: "observe", CompletionTokens: 8192, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"}, Count: 3},
		{Policy: llm.CallPolicy{Ref: cheapRef, Stage: "write", CompletionTokens: 32768, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"}, Count: 1},
	}}
}
func TestClipPricingRequiresKnownExactDecimalAndChecksOverflow(t *testing.T) {
	if got := boundedClipCharge(math.MaxInt64, 18); got != 18 {
		t.Fatal("overflowed settlement", got)
	}
	if got := boundedClipCharge(1, 2); got != 2 {
		t.Fatal("small ceiling", got)
	}
	for _, rate := range []string{"", "-1", "NaN", "Inf", "1/3", "1e999", "9999999999999999999999999999999"} {
		c := approvedTestClip().Calls
		c[0].Policy.InputUSDPerMillion = rate
		if _, err := ClipCredits(c); !errors.Is(err, ErrClipPricing) {
			t.Fatal(rate, err)
		}
	}
	for _, rate := range []string{"0", "0.000", "0e-9"} {
		c := approvedTestClip().Calls
		for i := range c {
			c[i].Policy.InputUSDPerMillion = rate
			c[i].Policy.OutputUSDPerMillion = rate
		}
		if credits, err := ClipCredits(c); err != nil || credits != 2 {
			t.Fatal(rate, credits, err)
		}
	}
	c := approvedTestClip().Calls
	credits, err := ClipCredits(c)
	if err != nil || credits != 18 {
		t.Fatal(credits, err)
	}
	c[0].Count = math.MaxInt
	if _, err = ClipCredits(c); !errors.Is(err, ErrClipPricing) {
		t.Fatal(err)
	}
}

func TestClipAdmissionRequiresCeilingAndNeverMutatesOnRefusal(t *testing.T) {
	for _, tier := range []plan.Plan{plan.Free, plan.Master} {
		svc, st := newTestService(t, seoulNoon)
		st.lots = []Lot{openMonthly("alice", 100)}
		start := holdStart("alice", tier, "clip")
		start.Kind = "generate_clip"
		if err := svc.Hold(context.Background(), start); !errors.Is(err, ErrClipApproval) {
			t.Fatal(err)
		}
		start.Clip = approvedTestClip()
		start.Clip.ApprovedMaxCredits = 0
		var exceeded *CreditCeilingError
		if err := svc.Hold(context.Background(), start); !errors.As(err, &exceeded) || exceeded.Approved != 0 || exceeded.Required != 18 {
			t.Fatal(err)
		}
		if st.balance("alice", seoulNoon) != 100 || len(st.admissions) != 0 {
			t.Fatal(st.lots, st.admissions)
		}
		start.Clip.ApprovedMaxCredits = 18
		if err := svc.Hold(context.Background(), start); err != nil {
			t.Fatal(err)
		}
		if st.admissions[0].ApprovedMaxCredits == nil || *st.admissions[0].ApprovedMaxCredits != 18 {
			t.Fatal(st.admissions)
		}
		st.events = append(st.events, Event{JobID: "clip", CostMicrousd: 1_000_000, CostSource: llm.CostReported})
		if err := svc.Settle(context.Background(), "clip", OutcomeSucceeded); err != nil {
			t.Fatal(err)
		}
		if st.settled["clip"] != 18 {
			t.Fatal(st.settled)
		}
		want := 82
		if tier == plan.Master {
			want = 100
		}
		if st.balance("alice", seoulNoon) != want {
			t.Fatal(st.lots)
		}
	}
}

func TestFrozenClipRatesSurviveCatalogChangeAndAreOwnerScoped(t *testing.T) {
	svc, st := newTestService(t, seoulNoon)
	p := approvedTestClip().Calls[0].Policy
	p.InputUSDPerMillion = "1.000"
	p.OutputUSDPerMillion = "2.000"
	ctx := WithCallPrice(context.Background(), "alice", "clip", p)
	call := Call{UserID: "alice", Kind: "generate_clip", JobID: "clip", Stage: "observe", Model: cheapRef, Usage: llm.Usage{PromptTokens: 1000, CompletionTokens: 1000}}
	if err := svc.Record(ctx, call); err != nil {
		t.Fatal(err)
	}
	if st.events[0].CostMicrousd != 3000 {
		t.Fatal(st.events)
	}
	call.UserID = "bob"
	if _, ok := frozenCallPrice(ctx, call); ok {
		t.Fatal("foreign price accepted")
	}
	call.UserID = "alice"
	call.Stage = "write"
	if _, ok := frozenCallPrice(ctx, call); ok {
		t.Fatal("wrong stage accepted")
	}
}
