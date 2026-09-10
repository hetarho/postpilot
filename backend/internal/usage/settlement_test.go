package usage

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

func TestClipFailedSettlementUsesOnlyConfirmedBillableEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []Event
		want   int
	}{
		{name: "no events"},
		{name: "unavailable", events: []Event{{CostSource: llm.CostUnavailable}}},
		{name: "unavailable is not billable even with stale amount", events: []Event{{CostSource: llm.CostUnavailable, CostMicrousd: 10000}}},
		{name: "reported zero overrides token estimate", events: []Event{{CostSource: llm.CostReported, PromptTokens: 30000, CompletionTokens: 8192}}},
		{name: "known free estimate", events: []Event{{CostSource: llm.CostEstimated, PromptTokens: 30}}},
		{name: "positive reported", events: []Event{{CostSource: llm.CostReported, CostMicrousd: 1}}, want: 3},
		{name: "positive estimated", events: []Event{{CostSource: llm.CostEstimated, PromptTokens: 100, CostMicrousd: 10000}}, want: 5},
		{name: "partial positive with unknown", events: []Event{{CostSource: llm.CostReported, CostMicrousd: 10000}, {CostSource: llm.CostUnavailable}}, want: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, store := newTestService(t, seoulNoon)
			store.lots = []Lot{openMonthly("alice", 100)}
			start := holdStart("alice", plan.Free, "clip", PlannedCall{Ref: cheapRef, Count: 10, CompletionTokens: 8192})
			start.Kind = "generate_clip"
			if err := svc.Hold(context.Background(), start); err != nil {
				t.Fatal(err)
			}
			if store.admissions[0].HoldCredits < tc.want {
				t.Fatal("fixture hold too small")
			}
			for _, event := range tc.events {
				event.JobID = "clip"
				store.events = append(store.events, event)
			}
			before := append([]Event(nil), store.events...)
			for range 2 {
				if err := svc.Settle(context.Background(), "clip", OutcomeFailed); err != nil {
					t.Fatal(err)
				}
			}
			if store.settled["clip"] != tc.want || store.balance("alice", seoulNoon) != 100-tc.want {
				t.Fatal(store.settled, store.lots)
			}
			if !reflect.DeepEqual(before, store.events) {
				t.Fatal("settlement rewrote raw provider evidence")
			}
			store.events = append(store.events, Event{JobID: "clip", CostSource: llm.CostReported, CostMicrousd: 100000})
			if err := svc.Settle(context.Background(), "clip", OutcomeFailed); err != nil {
				t.Fatal(err)
			}
			if store.settled["clip"] != tc.want || store.balance("alice", seoulNoon) != 100-tc.want {
				t.Fatal("late usage reopened settlement")
			}
		})
	}
}

func TestFailureWaiverIsClipAndOutcomeSpecific(t *testing.T) {
	for _, tc := range []struct {
		kind    string
		outcome TerminalOutcome
		tier    plan.Plan
		want    int
	}{
		{"generate_clip", OutcomeSucceeded, plan.Free, 2},
		{"generate", OutcomeFailed, plan.Free, 2},
		{"model_experiment", OutcomeFailed, plan.Free, 2},
		{"generate_clip", OutcomeFailed, plan.Master, 0},
		{"generate_clip", OutcomeSucceeded, plan.Master, 2},
	} {
		t.Run(tc.kind+string(tc.outcome)+string(tc.tier), func(t *testing.T) {
			svc, store := newTestService(t, seoulNoon)
			store.lots = []Lot{{ID: "purchased", UserID: "alice", Kind: LotPurchased, Granted: 100, Remaining: 100}}
			start := holdStart("alice", tc.tier, "job", PlannedCall{Ref: cheapRef, Count: 2})
			start.Kind = tc.kind
			if err := svc.Hold(context.Background(), start); err != nil {
				t.Fatal(err)
			}
			before := store.balance("alice", seoulNoon)
			if err := svc.Settle(context.Background(), "job", ""); !errors.Is(err, ErrSettlementOutcome) {
				t.Fatal(err)
			}
			if store.balance("alice", seoulNoon) != before || len(store.settled) != 0 {
				t.Fatal("unknown outcome mutated accounting")
			}
			if err := svc.Settle(context.Background(), "job", tc.outcome); err != nil {
				t.Fatal(err)
			}
			if store.settled["job"] != tc.want {
				t.Fatal(store.settled)
			}
			if tc.tier == plan.Master && store.lots[0].Remaining != 100 {
				t.Fatal("master debited")
			}
		})
	}
}

func TestFailedCallPreservesReportedZeroForSettlement(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 100)}
	start := holdStart("alice", plan.Free, "clip", PlannedCall{Ref: cheapRef, Count: 2})
	start.Kind = "generate_clip"
	if err := svc.Hold(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	ctx := WithWork(context.Background(), Work{UserID: "alice", Kind: "generate_clip", JobID: "clip"})
	if err := svc.RecordCall(ctx, cheapRef, "observe", llm.Usage{PromptTokens: 30000, CompletionTokens: 8192, CostReported: true}, errors.New("provider failure")); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 1 || store.events[0].CostSource != llm.CostReported || store.events[0].CostMicrousd != 0 {
		t.Fatal(store.events)
	}
	if err := svc.Settle(ctx, "clip", OutcomeFailed); err != nil {
		t.Fatal(err)
	}
	if store.settled["clip"] != 0 || store.balance("alice", seoulNoon) != 100 {
		t.Fatal(store.settled, store.lots)
	}
}
