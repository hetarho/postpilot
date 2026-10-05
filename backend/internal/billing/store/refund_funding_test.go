package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// billingCoverage is the ledger's coverage source read from billing itself, as the API wires
// it: a lazily issued grant carries the tier billing says the account held that day.
type billingCoverage struct{ service *billing.Service }

func (c billingCoverage) AnchorFor(ctx context.Context, userID string) (time.Time, error) {
	at, _, err := c.service.AnchorFor(ctx, userID)
	return at, err
}

func (c billingCoverage) CoverageFor(ctx context.Context, userID string, at time.Time) (usage.Coverage, bool, error) {
	coverage, found, err := c.service.CoverageAt(ctx, userID, at)
	return usage.Coverage{ID: coverage.ID, Anchor: coverage.Anchor, End: coverage.End,
		Tier: coverage.Tier, DailyTier: coverage.DailyTier}, found, err
}

func fundingStamp(at time.Time) string { return at.UTC().Format("2006-01-02T15:04:05.000000000Z07:00") }

// fundedPayment is one refundable payment and the grants the real flow issued that it funded.
type fundedPayment struct {
	order   string
	lots    []string // credit lot ids
	windows []string // export window starts on the payment's coverage
}

// grantCandidate is a credit lot that may still be issued after a refund was approved.
type grantCandidate struct {
	id, kind, cause, coverage, correlation string
	start                                  time.Time
}

// grantCandidates spans every field the funded rule reads: each daily and monthly issuance
// cause on both sides of both window bounds and on the bounds themselves, another coverage,
// grants correlated to the order, and grants with no window.
func grantCandidates(order, coverage string, start, end time.Time) []grantCandidate {
	var out []grantCandidate
	add := func(kind, cause, cov, correlation string, at time.Time) {
		out = append(out, grantCandidate{id: fmt.Sprintf("candidate-%02d", len(out)), kind: kind,
			cause: cause, coverage: cov, correlation: correlation, start: at})
	}
	for _, kind := range []string{"daily", "monthly"} {
		for _, cause := range []string{"lazy", "coverage", "upgrade", ""} {
			for _, at := range []time.Time{start.Add(-90 * time.Minute), start.Add(90 * time.Minute),
				end.Add(-90 * time.Minute), end.Add(90 * time.Minute)} {
				// A second apart, so no two share a grant window.
				add(kind, cause, coverage, "", at.Add(time.Duration(len(out))*time.Second))
			}
		}
	}
	// The bounds themselves, with causes the grant-window index leaves free.
	add("daily", "", coverage, "", start)
	add("monthly", "upgrade", coverage, "", start)
	add("daily", "", coverage, "", end)
	add("monthly", "upgrade", coverage, "", end)
	add("daily", "lazy", "other-coverage", "", start.Add(2*time.Hour))
	add("monthly", "upgrade", coverage, order, start.Add(3*time.Hour))
	add("bonus", "", "", order, time.Time{})
	add("purchased", "", "", "", time.Time{})
	return out
}

// windowCandidate is an export window that may still open after a refund was approved.
type windowCandidate struct {
	coverage string
	start    time.Time
}

func (w windowCandidate) key() string { return w.coverage + "@" + fundingStamp(w.start) }

// windowCandidates sit on both sides of both window bounds, on the end bound, and on another
// coverage.
func windowCandidates(coverage string, start, end time.Time) []windowCandidate {
	var out []windowCandidate
	for index, at := range []time.Time{start.Add(-90 * time.Minute), start.Add(90 * time.Minute),
		end.Add(-90 * time.Minute), end.Add(90 * time.Minute)} {
		out = append(out, windowCandidate{coverage, at.Add(time.Duration(index) * time.Second)})
	}
	return append(out, windowCandidate{coverage, end}, windowCandidate{"other-coverage", start.Add(2 * time.Hour)})
}

func nullText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func issueCandidateLot(t *testing.T, h *ledgerHarness, c grantCandidate, expires time.Time) {
	t.Helper()
	var start any
	if !c.start.IsZero() {
		start = fundingStamp(c.start)
	}
	if _, err := h.handle.Writer.Exec(`INSERT INTO credit_lots
		(id,user_id,kind,granted,remaining,expires_at,created_at,coverage_id,window_start,issuance_cause,correlation_id)
		VALUES (?,'alice',?,7,7,?,?,?,?,?,?)`, c.id, c.kind, fundingStamp(expires), fundingStamp(expires.Add(-time.Hour)),
		nullText(c.coverage), start, nullText(c.cause), nullText(c.correlation)); err != nil {
		t.Fatalf("issue %+v: %v", c, err)
	}
}

func openCandidateWindow(t *testing.T, h *ledgerHarness, w windowCandidate) {
	t.Helper()
	if _, err := h.handle.Writer.Exec(`INSERT INTO server_export_windows
		(user_id,coverage_id,window_start,window_end,allowance) VALUES ('alice',?,?,?,5)`,
		w.coverage, fundingStamp(w.start), fundingStamp(w.start.Add(time.Hour))); err != nil {
		t.Fatalf("open window %v: %v", w, err)
	}
}

func idsWhere(t *testing.T, h *ledgerHarness, query string, args ...any) []string {
	t.Helper()
	rows, err := h.handle.Reader.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func lotExists(t *testing.T, h *ledgerHarness, id string) bool {
	t.Helper()
	return len(idsWhere(t, h, `SELECT id FROM credit_lots WHERE id=?`, id)) == 1
}

func windowExists(t *testing.T, h *ledgerHarness, w windowCandidate) bool {
	t.Helper()
	return len(idsWhere(t, h, `SELECT window_start FROM server_export_windows
		WHERE user_id='alice' AND coverage_id=? AND window_start=?`, w.coverage, fundingStamp(w.start))) == 1
}

// F37: what a refunded payment funded is one value billing computes, read alike by the refund
// evidence and by the guard. For each kind of payment, every grant the guard keeps from being
// issued is one the evidence counts as funded, and every grant the evidence counts is one the
// guard keeps from being issued.
func TestRefundEvidenceAndGuardAgreeOnWhatAPaymentFunded(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 3, 10, 3, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		pay  func(t *testing.T, h *ledgerHarness, clock *time.Time) fundedPayment
	}{
		{"subscribe", func(t *testing.T, h *ledgerHarness, _ *time.Time) fundedPayment {
			sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
			if err != nil {
				t.Fatal(err)
			}
			return fundedPayment{order: chargeOrder(t, h, "subscribe"),
				lots:    idsWhere(t, h, `SELECT id FROM credit_lots WHERE coverage_id=?`, sub.CoverageID),
				windows: []string{fundingStamp(sub.TermStart)}}
		}},
		{"renew", func(t *testing.T, h *ledgerHarness, clock *time.Time) fundedPayment {
			sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
			if err != nil {
				t.Fatal(err)
			}
			*clock = sub.TermEnd
			if err := h.service.RunDue(ctx, *clock); err != nil {
				t.Fatal(err)
			}
			return fundedPayment{order: chargeOrder(t, h, "renew"),
				lots: idsWhere(t, h, `SELECT id FROM credit_lots WHERE coverage_id=? AND window_start>=?`,
					sub.CoverageID, fundingStamp(sub.TermEnd)),
				windows: []string{fundingStamp(sub.TermEnd)}}
		}},
		{"pack", func(t *testing.T, h *ledgerHarness, _ *time.Time) fundedPayment {
			if _, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); err != nil {
				t.Fatal(err)
			}
			pack, err := h.service.PurchasePack(ctx, "alice", "pack-1000")
			if err != nil {
				t.Fatal(err)
			}
			return fundedPayment{order: pack.OrderID, lots: []string{pack.LotID}}
		}},
		{"upgrade whose window issued a lazy daily lot", func(t *testing.T, h *ledgerHarness, clock *time.Time) fundedPayment {
			sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermAnnual)
			if err != nil {
				t.Fatal(err)
			}
			*clock = at.Add(2*24*time.Hour + time.Hour)
			quote, err := h.service.QuoteChange(ctx, "alice", plan.Pro, billing.TermAnnual)
			if err != nil {
				t.Fatal(err)
			}
			if _, applied, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Pro, billing.TermAnnual, quote.ID); err != nil || !applied {
				t.Fatalf("upgrade applied=%t err=%v", applied, err)
			}
			order := chargeOrder(t, h, "upgrade")
			// The next day the ledger lazily issues that day's grant at the tier the upgrade raised.
			*clock = clock.Add(24 * time.Hour)
			ledger := usage.NewService(usagestore.New(h.handle.Writer, h.handle.Reader), nil, 0,
				billingCoverage{h.service}, testRates).WithClock(func() time.Time { return *clock })
			if _, err := ledger.BalanceFor(ctx, "alice", plan.Pro); err != nil {
				t.Fatal(err)
			}
			lazy := idsWhere(t, h, `SELECT id FROM credit_lots WHERE coverage_id=? AND kind='daily'
				AND issuance_cause='lazy' AND window_start>=?`, sub.CoverageID, fundingStamp(quote.EffectiveAt))
			pro, _ := plan.CommercialOffer(plan.Pro)
			var granted int
			if len(lazy) != 1 {
				t.Fatalf("lazy daily lots in the upgrade's window = %v", lazy)
			}
			if err := h.handle.Reader.QueryRow(`SELECT granted FROM credit_lots WHERE id=?`, lazy[0]).Scan(&granted); err != nil || granted != pro.DailyCredits {
				t.Fatalf("lazy daily grant=%d err=%v, want the raised tier's %d", granted, err, pro.DailyCredits)
			}
			lots := append(lazy, idsWhere(t, h, `SELECT id FROM credit_lots WHERE correlation_id=?`, order)...)
			slices.Sort(lots)
			return fundedPayment{order: order, lots: lots, windows: []string{fundingStamp(sub.TermStart)}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, provider, clock := refundHarness(t, at)
			paid := tc.pay(t, h, clock)
			payment, found, err := h.store.RefundPayment(ctx, "alice", paid.order)
			if err != nil || !found {
				t.Fatalf("refundable payment found=%t err=%v", found, err)
			}
			credit, export := refundTestBenefits{}.credit(payment), refundTestBenefits{}.export(payment)
			credits := usagestore.New(h.handle.Writer, h.handle.Reader)
			exports := clipstore.New(h.handle.Writer, h.handle.Reader)

			request, err := h.service.RequestRefund(ctx, "alice", paid.order, "funded set")
			if err != nil {
				t.Fatal(err)
			}
			var remaining int
			for _, id := range paid.lots {
				var lot int
				if err := h.handle.Reader.QueryRow(`SELECT remaining FROM credit_lots WHERE id=?`, id).Scan(&lot); err != nil {
					t.Fatal(err)
				}
				remaining += lot
			}
			if request.Evidence.FundedCreditsRemaining != remaining {
				t.Fatalf("evidence counts %d funded credits, the payment's grants %v hold %d",
					request.Evidence.FundedCreditsRemaining, paid.lots, remaining)
			}
			// The guard stays up while the provider's answer is lost.
			provider.timeout = true
			if reviewed, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW); err != nil || reviewed.Status != "processing" {
				t.Fatalf("review=%+v err=%v", reviewed, err)
			}
			if frozen := idsWhere(t, h, `SELECT id FROM credit_lots WHERE refund_request_id=?`, request.ID); !slices.Equal(frozen, paid.lots) {
				t.Fatalf("frozen lots = %v, want %v", frozen, paid.lots)
			}
			if frozen := idsWhere(t, h, `SELECT window_start FROM server_export_windows WHERE refund_request_id=?`, request.ID); !slices.Equal(frozen, paid.windows) {
				t.Fatalf("frozen windows = %v, want %v", frozen, paid.windows)
			}

			coverage, start, end := credit.CoverageID, credit.Start, credit.End
			if coverage == "" {
				// A pack funds no window: probe its subscription's around the purchase.
				sub, _, err := h.store.Subscription(ctx, "alice")
				if err != nil {
					t.Fatal(err)
				}
				coverage, start, end = sub.CoverageID, payment.ChargedAt, payment.ChargedAt.Add(30*24*time.Hour)
			}
			lots, windows := grantCandidates(paid.order, coverage, start, end), windowCandidates(coverage, start, end)
			expires := end.Add(365 * 24 * time.Hour)

			// Under the guard: whatever is still issued, the evidence's predicate does not take.
			for _, lot := range lots {
				issueCandidateLot(t, h, lot, expires)
			}
			for _, window := range windows {
				openCandidateWindow(t, h, window)
			}
			var blockedLots, blockedWindows []string
			var blockedLotCandidates []grantCandidate
			var blockedWindowCandidates []windowCandidate
			for _, lot := range lots {
				if !lotExists(t, h, lot.id) {
					blockedLots = append(blockedLots, lot.id)
					blockedLotCandidates = append(blockedLotCandidates, lot)
				}
			}
			for _, window := range windows {
				if !windowExists(t, h, window) {
					blockedWindows = append(blockedWindows, window.key())
					blockedWindowCandidates = append(blockedWindowCandidates, window)
				}
			}
			if err := credits.GuardRefundFunding(ctx, credit, "probe-issued"); err != nil {
				t.Fatal(err)
			}
			if err := exports.GuardRefundFunding(ctx, export, "probe-issued"); err != nil {
				t.Fatal(err)
			}
			if funded := idsWhere(t, h, `SELECT id FROM credit_lots WHERE refund_request_id='probe-issued'`); len(funded) != 0 {
				t.Fatalf("lots the guard let through but the evidence counts as funded: %v", funded)
			}
			if funded := idsWhere(t, h, `SELECT coverage_id||'@'||window_start FROM server_export_windows
				WHERE refund_request_id='probe-issued'`); len(funded) != 0 {
				t.Fatalf("windows the guard let open but the evidence counts as funded: %v", funded)
			}

			// With every guard lifted: whatever the guard had blocked, the evidence's predicate takes.
			for _, table := range []string{"credit_refund_funding_guards", "server_export_refund_guards"} {
				if _, err := h.handle.Writer.Exec(`DELETE FROM ` + table); err != nil {
					t.Fatal(err)
				}
			}
			for _, lot := range blockedLotCandidates {
				issueCandidateLot(t, h, lot, expires)
			}
			for _, window := range blockedWindowCandidates {
				openCandidateWindow(t, h, window)
			}
			if err := credits.GuardRefundFunding(ctx, credit, "probe-blocked"); err != nil {
				t.Fatal(err)
			}
			if err := exports.GuardRefundFunding(ctx, export, "probe-blocked"); err != nil {
				t.Fatal(err)
			}
			if funded := idsWhere(t, h, `SELECT id FROM credit_lots WHERE refund_request_id='probe-blocked'`); !slices.Equal(funded, blockedLots) {
				t.Fatalf("funded lots %v, guard-blocked lots %v", funded, blockedLots)
			}
			funded := idsWhere(t, h, `SELECT coverage_id||'@'||window_start FROM server_export_windows
				WHERE refund_request_id='probe-blocked'`)
			slices.Sort(blockedWindows)
			if !slices.Equal(funded, blockedWindows) {
				t.Fatalf("funded windows %v, guard-blocked windows %v", funded, blockedWindows)
			}
			if payment.Kind != "pack" && (len(blockedLots) == 0 || len(blockedWindows) == 0) {
				t.Fatalf("a %s guard blocked nothing: lots=%v windows=%v", payment.Kind, blockedLots, blockedWindows)
			}

			// The evidence's count is the frozen set's.
			now := *clock
			evidence, err := credits.RefundFundingEvidence(ctx, credit, now)
			if err != nil {
				t.Fatal(err)
			}
			var frozen sql.NullInt64
			if err := h.handle.Reader.QueryRow(`SELECT SUM(remaining) FROM credit_lots
				WHERE refund_request_id IN (?,'probe-blocked') AND (expires_at IS NULL OR expires_at>?)`,
				request.ID, fundingStamp(now)).Scan(&frozen); err != nil {
				t.Fatal(err)
			}
			if evidence.CreditsRemaining != int(frozen.Int64) {
				t.Fatalf("evidence remaining=%d, frozen remaining=%d", evidence.CreditsRemaining, frozen.Int64)
			}
		})
	}
}
