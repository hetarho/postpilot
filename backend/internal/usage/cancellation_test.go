package usage

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

func TestCancelledClipChargeUsesTheUnusedReservationAndCannotOverflow(t *testing.T) {
	for _, tc := range []struct {
		reservation    int
		cost           int64
		confirmed, fee int
	}{
		{100, 60000, 20, 40}, {5, 0, 0, 3}, {5, 100, 3, 1},
		{0, 100, 0, 0}, {5, math.MaxInt64, 5, 0},
		{math.MaxInt, 0, 0, math.MaxInt/2 + 1},
	} {
		confirmed, fee := cancelledCharge(tc.cost, tc.reservation)
		if confirmed != tc.confirmed || fee != tc.fee || confirmed+fee > tc.reservation {
			t.Fatalf("reservation %d cost %d: confirmed=%d fee=%d", tc.reservation, tc.cost, confirmed, fee)
		}
	}
}

// TMPL-63, QUOTA-49: work admitted without an approved ceiling settles as cancelled only for a
// kind the root names, and then it is charged exactly what its confirmed usage costs — the same
// as a finished run of that usage, with no cancellation fee — and the rest of the hold returns.
func TestOwnerCancellableWorkSettlesToConfirmedUsageWithoutAFee(t *testing.T) {
	settle := func(t *testing.T, owner bool, outcome TerminalOutcome) (Settlement, int, error) {
		t.Helper()
		svc, st := newTestService(t, seoulNoon)
		if owner {
			svc.WithOwnerCancellation("template_request")
		}
		st.lots = []Lot{openMonthly("alice", 1000)}
		start := holdStart("alice", plan.Free, "job")
		start.Kind = "template_request"
		if err := svc.Hold(context.Background(), start); err != nil {
			t.Fatal(err)
		}
		hold := st.admissions[0].HoldCredits
		st.events = append(st.events, Event{
			UserID: "alice", JobID: "job", Kind: "template_request", Model: cheapRef.String(),
			CostMicrousd: 10_000, CostSource: llm.CostReported,
		})
		err := svc.Settle(context.Background(), "job", outcome)
		return st.settlements["job"], hold, err
	}

	if _, _, err := settle(t, false, OutcomeCancelled); !errors.Is(err, ErrSettlementOutcome) {
		t.Fatalf("unnamed kind: err = %v, want ErrSettlementOutcome", err)
	}
	cancelled, hold, err := settle(t, true, OutcomeCancelled)
	if err != nil {
		t.Fatal(err)
	}
	finished, _, err := settle(t, true, OutcomeSucceeded)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Credits != finished.Credits {
		t.Fatalf("cancelled charge = %d, finished = %d (hold %d)", cancelled.Credits, finished.Credits, hold)
	}
	if cancelled.CancellationFee != nil && *cancelled.CancellationFee != 0 {
		t.Fatalf("cancellation fee = %d, want none", *cancelled.CancellationFee)
	}
}
