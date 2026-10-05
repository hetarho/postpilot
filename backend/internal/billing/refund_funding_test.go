package billing

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// F37: what a refundable payment funded is one value, derived here from the payment alone.
func TestRefundFundingIsDerivedOnceFromThePayment(t *testing.T) {
	start := time.Date(2026, 1, 31, 3, 0, 0, 0, time.UTC)
	termEnd := TermEnd(start, start, TermMonthly)
	recorded := start.AddDate(0, 0, 40)
	renewal := start.AddDate(0, 0, 20)
	for _, tc := range []struct {
		name    string
		payment RefundPayment
		want    RefundFunding
	}{
		{"a pack funds its purchased lot alone",
			RefundPayment{OrderID: "buy", UserID: "alice", Kind: "pack", PackLotID: "lot-1", CoverageID: "ignored",
				EffectiveAt: start, FundedTermEnd: recorded},
			RefundFunding{UserID: "alice", OrderID: "buy", Kind: "pack", LotID: "lot-1"}},
		{"a subscription funds its coverage to the recorded term end",
			RefundPayment{OrderID: "sub", UserID: "alice", Kind: "subscribe", CoverageID: "cov", Term: TermMonthly,
				EffectiveAt: start, FundedTermEnd: recorded},
			RefundFunding{UserID: "alice", OrderID: "sub", Kind: "subscribe", CoverageID: "cov", Start: start, End: recorded}},
		{"a row without a recorded end funds one term",
			RefundPayment{OrderID: "ren", UserID: "alice", Kind: "renew", CoverageID: "cov", Term: TermMonthly,
				EffectiveAt: start},
			RefundFunding{UserID: "alice", OrderID: "ren", Kind: "renew", CoverageID: "cov", Start: start, End: termEnd}},
		{"a later base payment cuts the window where its term begins",
			RefundPayment{OrderID: "sub", UserID: "alice", Kind: "subscribe", CoverageID: "cov", Term: TermMonthly,
				EffectiveAt: start, FundedTermEnd: recorded, NextBaseStart: renewal},
			RefundFunding{UserID: "alice", OrderID: "sub", Kind: "subscribe", CoverageID: "cov", Start: start, End: renewal}},
		{"a base payment starting at or after the end leaves it",
			RefundPayment{OrderID: "sub", UserID: "alice", Kind: "subscribe", CoverageID: "cov", Term: TermMonthly,
				EffectiveAt: start, FundedTermEnd: renewal, NextBaseStart: renewal},
			RefundFunding{UserID: "alice", OrderID: "sub", Kind: "subscribe", CoverageID: "cov", Start: start, End: renewal}},
		{"an upgrade funds its correlated grants and the lazily issued ones in its window",
			RefundPayment{OrderID: "upg", UserID: "alice", Kind: "upgrade", CoverageID: "cov", Term: TermMonthly,
				Tier: plan.Pro, PriorTier: plan.Basic, EffectiveAt: start, FundedTermEnd: recorded, NextBaseStart: renewal},
			RefundFunding{UserID: "alice", OrderID: "upg", Kind: "upgrade", Correlation: "upg", CoverageID: "cov",
				WindowCause: "lazy", Start: start, End: renewal}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.payment.Funding(); got != tc.want {
				t.Fatalf("Funding() = %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
