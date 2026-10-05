package usage

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// testRate is the snapshot testRates selects, frozen into the approval of approved work.
var testRate = plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-14",
	ReferenceE4: testReferenceE4, AppliedE4: testReferenceE4}

// approvedHold is what approvedTestClip reserves: 52 143 micro-USD at 500 KRW per USD.
const approvedHold = 27

func approvedTestClip() *Reservation {
	return &Reservation{ApprovedMaxCredits: 100, Rate: testRate, Calls: []PricedCall{
		{Policy: llm.CallPolicy{Ref: cheapRef, Stage: "observe", CompletionTokens: 8192, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"}, Count: 3},
		{Policy: llm.CallPolicy{Ref: cheapRef, Stage: "write", CompletionTokens: 32768, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"}, Count: 1},
	}}
}

func TestMultimodalAdmissionEnvelopeIsNeverMeasuredUsage(t *testing.T) {
	svc, st := newTestService(t, seoulNoon)
	p := approvedTestClip().Calls[0].Policy
	p.InputUSDPerMillion, p.OutputUSDPerMillion = "1", "2.5"
	p.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionInlineStatic, Endpoint: "leaf", RequiredParameters: "max_tokens", PromptUSDPerMillion: "0.3", CompletionUSDPerMillion: "2.5", RequestUSD: "0.01", ImageUSD: "0.0000003", AudioUSDPerToken: "0.000001"}
	if cost, ok := p.QuoteMicrousd(); !ok || cost != 60498 {
		t.Fatalf("quote %d valid %v", cost, ok)
	}
	ctx := WithCallPrice(context.Background(), "alice", "clip", p)
	call := Call{UserID: "alice", Kind: "generate_clip", JobID: "clip", Stage: "observe", Model: cheapRef, Usage: llm.Usage{PromptTokens: 1000, CompletionTokens: 1000}}
	for _, evidence := range []struct {
		reported bool
		amount   int64
		source   llm.CostSource
	}{
		{false, 0, llm.CostUnavailable}, {true, 0, llm.CostReported}, {true, 321, llm.CostReported},
	} {
		call.Usage.CostReported, call.Usage.CostMicrousd = evidence.reported, evidence.amount
		if err := svc.Record(ctx, call); err != nil {
			t.Fatal(err)
		}
		event := st.events[len(st.events)-1]
		if event.CostSource != evidence.source || event.CostMicrousd != evidence.amount || event.PromptTokens != 1000 {
			t.Fatalf("fabricated usage: %+v", event)
		}
	}
	// A uniform text-only profile still permits an evidenced aggregate estimate.
	p.Pricing.Delivery = llm.ExecutionTextOnly
	p.Pricing.AggregateUsageSufficient = true
	p.Pricing.RequestUSD = "0"
	call.Usage.CostReported = false
	ctx = WithCallPrice(context.Background(), "alice", "clip", p)
	if err := svc.Record(ctx, call); err != nil {
		t.Fatal(err)
	}
	event := st.events[len(st.events)-1]
	if event.CostSource != llm.CostEstimated || event.CostMicrousd != 3500 {
		t.Fatal(event)
	}
}
func TestReservationPricingRequiresKnownExactDecimalARateAndChecksOverflow(t *testing.T) {
	for _, rate := range []string{"", "-1", "NaN", "Inf", "1/3", "1e999", "9999999999999999999999999999999"} {
		c := approvedTestClip().Calls
		c[0].Policy.InputUSDPerMillion = rate
		if _, err := ReservationCreditsAt(c, testRate); !errors.Is(err, ErrPricingUnavailable) {
			t.Fatal(rate, err)
		}
	}
	// A free reservation holds nothing, with or without a rate.
	for _, rate := range []string{"0", "0.000", "0e-9"} {
		c := approvedTestClip().Calls
		for i := range c {
			c[i].Policy.InputUSDPerMillion = rate
			c[i].Policy.OutputUSDPerMillion = rate
		}
		if credits, err := ReservationCreditsAt(c, plan.RateSnapshot{}); err != nil || credits != 0 {
			t.Fatal(rate, credits, err)
		}
	}
	c := approvedTestClip().Calls
	credits, err := ReservationCreditsAt(c, testRate)
	if err != nil || credits != approvedHold {
		t.Fatal(credits, err)
	}
	if _, err := ReservationCreditsAt(c, plan.RateSnapshot{}); !errors.Is(err, ErrPricingUnavailable) {
		t.Fatal("a paid reservation was priced without a rate", err)
	}
	c[0].Count = math.MaxInt
	if _, err = ReservationCreditsAt(c, testRate); !errors.Is(err, ErrPricingUnavailable) {
		t.Fatal(err)
	}
}

func TestClipAdmissionRequiresCeilingAndNeverMutatesOnRefusal(t *testing.T) {
	for _, tier := range []plan.Plan{plan.Free, plan.Master} {
		svc, st := newTestService(t, seoulNoon)
		st.lots = []Lot{openMonthly("alice", 100)}
		start := holdStart("alice", tier, "clip")
		start.Kind = "generate_clip"
		if err := svc.Hold(context.Background(), start); !errors.Is(err, ErrApprovalRequired) {
			t.Fatal(err)
		}
		start.Approval = approvedTestClip()
		start.Approval.ApprovedMaxCredits = 0
		var exceeded *CreditCeilingError
		if err := svc.Hold(context.Background(), start); !errors.As(err, &exceeded) || exceeded.Approved != 0 || exceeded.Required != approvedHold {
			t.Fatal(err)
		}
		if st.balance("alice", seoulNoon) != 100 || len(st.admissions) != 0 {
			t.Fatal(st.lots, st.admissions)
		}
		// A paid approval that froze no rate cannot be converted.
		start.Approval.ApprovedMaxCredits = approvedHold
		start.Approval.Rate = plan.RateSnapshot{}
		if err := svc.Hold(context.Background(), start); !errors.Is(err, ErrRateUnavailable) || len(st.admissions) != 0 {
			t.Fatal(err)
		}
		start.Approval.Rate = testRate
		if err := svc.Hold(context.Background(), start); err != nil {
			t.Fatal(err)
		}
		if st.admissions[0].ApprovedMaxCredits == nil || *st.admissions[0].ApprovedMaxCredits != approvedHold || st.admissions[0].Rate != testRate {
			t.Fatal(st.admissions)
		}
		st.events = append(st.events, Event{JobID: "clip", CostMicrousd: 1_000_000, CostSource: llm.CostReported})
		if err := svc.Settle(context.Background(), "clip", OutcomeSucceeded); err != nil {
			t.Fatal(err)
		}
		if st.settled["clip"] != approvedHold {
			t.Fatal(st.settled)
		}
		want := 100 - approvedHold
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

func TestMultimodalQuoteAndActualCountHoldUseIdenticalPrices(t *testing.T) {
	svc, st := newTestService(t, seoulNoon)
	st.lots = []Lot{openMonthly("alice", 1000)}
	reservation := approvedTestClip()
	for i := range reservation.Calls {
		p := &reservation.Calls[i].Policy
		p.InputUSDPerMillion, p.OutputUSDPerMillion = "1", "2.5"
		p.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionTextOnly, Endpoint: "leaf", RequiredParameters: "max_tokens", PromptUSDPerMillion: "0.3", CompletionUSDPerMillion: "2.5", RequestUSD: "0.01", ImageUSD: "0.0000003", AudioUSDPerToken: "0.000001"}
	}
	reservation.Calls[0].Policy.Pricing.Delivery = llm.ExecutionInlineStatic
	quoted, err := ReservationCreditsAt(reservation.Calls, reservation.Rate)
	if err != nil {
		t.Fatal(err)
	}
	reservation.ApprovedMaxCredits = quoted
	reservation.Calls[0].Count--
	actual, err := ReservationCreditsAt(reservation.Calls, reservation.Rate)
	if err != nil || actual >= quoted {
		t.Fatal(actual, quoted, err)
	}
	start := holdStart("alice", plan.Free, "multimodal")
	start.Kind, start.Approval = "generate_clip", reservation
	if err = svc.Hold(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	if st.balance("alice", seoulNoon) != 1000-actual {
		t.Fatal("hold differs from shared calculator", st.lots)
	}
	if err = svc.Settle(context.Background(), "multimodal", OutcomeFailed); err != nil {
		t.Fatal(err)
	}
	if st.balance("alice", seoulNoon) != 1000 {
		t.Fatal("unused failure was not fully refunded", st.lots)
	}
}
