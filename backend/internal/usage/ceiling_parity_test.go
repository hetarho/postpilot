package usage

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// The charge every approved settlement produces, walked across the four inputs that
// decide it. These numbers are what the ledger charged when the rules were keyed on the
// clip kinds; the rename must not move one of them.
func TestApprovedSettlementChargesAreUnchanged(t *testing.T) {
	const hold = 18 // what approvedTestClip's three observations and one writing call reserve
	for _, tc := range []struct {
		name          string
		outcome       TerminalOutcome
		confirmed     int64 // confirmed provider cost in micro-USD
		ceiling       int
		policyVersion int
		charge        int // credits taken from the account
		fee           int // the cancellation fee inside that charge
	}{
		{name: "success below the ceiling", outcome: OutcomeSucceeded, confirmed: 10_000, ceiling: 100, charge: 5},
		{name: "success capped by the approval", outcome: OutcomeSucceeded, confirmed: 10_000_000, ceiling: 100, charge: hold},
		{name: "success with no confirmed cost still pays the base", outcome: OutcomeSucceeded, confirmed: 0, ceiling: 100, charge: plan.ChargeBase},
		{name: "failure with no confirmed cost waives even the base", outcome: OutcomeFailed, confirmed: 0, ceiling: 100, charge: 0},
		{name: "failure with confirmed cost is charged", outcome: OutcomeFailed, confirmed: 10_000, ceiling: 100, charge: 5},
		{name: "cancellation halves the unused hold as its fee", outcome: OutcomeCancelled, confirmed: 10_000, ceiling: 100, policyVersion: 1, charge: 5 + (hold-5)/2 + (hold-5)%2, fee: (hold-5)/2 + (hold-5)%2},
		{name: "cancellation before any cost is fee only", outcome: OutcomeCancelled, confirmed: 0, ceiling: 100, policyVersion: 1, charge: hold/2 + hold%2, fee: hold/2 + hold%2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, st := newTestService(t, seoulNoon)
			st.lots = []Lot{openMonthly("alice", 1000)}
			start := holdStart("alice", plan.Free, "job")
			start.Kind = "generate_clip"
			start.Approval = approvedTestClip()
			start.Approval.ApprovedMaxCredits = tc.ceiling
			start.Approval.CancellationPolicyVersion = tc.policyVersion
			if err := svc.Hold(context.Background(), start); err != nil {
				t.Fatal(err)
			}
			if len(st.admissions) != 1 || st.admissions[0].HoldCredits != hold {
				t.Fatalf("hold = %v, want %d", st.admissions, hold)
			}
			if tc.confirmed > 0 {
				st.events = append(st.events, Event{
					UserID: "alice", JobID: "job", Kind: "generate_clip", Model: cheapRef.String(),
					CostMicrousd: tc.confirmed, CostSource: llm.CostReported,
				})
			}
			if err := svc.Settle(context.Background(), "job", tc.outcome); err != nil {
				t.Fatal(err)
			}
			settled := st.settlements["job"]
			if settled.Credits != tc.charge {
				t.Fatalf("charge = %d, want %d", settled.Credits, tc.charge)
			}
			if tc.fee > 0 && (settled.CancellationFee == nil || *settled.CancellationFee != tc.fee) {
				t.Fatalf("cancellation fee = %v, want %d", settled.CancellationFee, tc.fee)
			}
			if tc.outcome != OutcomeCancelled && settled.CancellationFee != nil && *settled.CancellationFee != 0 {
				t.Fatalf("a non-cancellation carried a fee: %v", settled.CancellationFee)
			}
		})
	}
}

// Work of a kind the root did not mark may not carry an approval into a ceiling
// settlement, and work that must be approved may not start without one.
func TestApprovalIsRequiredExactlyForTheKindsTheRootNamed(t *testing.T) {
	svc, st := newTestService(t, seoulNoon)
	st.lots = []Lot{openMonthly("alice", 1000)}
	unapproved := holdStart("alice", plan.Free, "job", PlannedCall{Ref: cheapRef, Count: 1, CompletionTokens: maxCompletion})
	unapproved.Kind = "generate_clip"
	if err := svc.Hold(context.Background(), unapproved); err == nil {
		t.Fatal("approved work started without its approval")
	}
	ordinary := holdStart("alice", plan.Free, "ordinary", PlannedCall{Ref: cheapRef, Count: 1, CompletionTokens: maxCompletion})
	if err := svc.Hold(context.Background(), ordinary); err != nil {
		t.Fatal(err)
	}
	for _, admission := range st.admissions {
		if admission.JobID == "ordinary" && admission.ApprovedMaxCredits != nil {
			t.Fatal("ordinary work recorded an approval it never made")
		}
	}
}
