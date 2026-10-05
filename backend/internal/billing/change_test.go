package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// quoteAndChange confirms a change through the quote it was priced by, the only way a change
// reaches the card.
func quoteAndChange(ctx context.Context, t *testing.T, service *Service, tier plan.Plan, term Term) (ChangeQuote, Subscription, bool, error) {
	t.Helper()
	quote, err := service.QuoteChange(ctx, "alice", tier, term)
	if err != nil {
		t.Fatalf("quote %s %s: %v", tier, term, err)
	}
	updated, applied, err := service.ChangeSubscriptionQuoted(ctx, "alice", tier, term, quote.ID)
	return quote, updated, applied, err
}

func TestChangeSubscriptionClassifiesAndChargesOnlyUpgrades(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, seoul)
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	termStart := time.Date(2026, 5, 15, 0, 0, 0, 0, seoul)
	termEnd := time.Date(2026, 6, 15, 0, 0, 0, 0, seoul)
	monthly := func(tier plan.Plan) Subscription {
		subscription := activeSubscription("alice", tier, TermMonthly, anchor, termEnd, termEnd, true)
		subscription.TermStart = termStart
		return subscription
	}

	t.Run("refusals use the shared classifier", func(t *testing.T) {
		store := newSubscriptionStore()
		service := newSubscriptionService(store, newSubscriptionProvider(), now)
		if _, err := service.QuoteChange(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrSubscriptionRequired) {
			t.Fatalf("missing subscription = %v", err)
		}
		store.subscriptions["alice"] = monthly(plan.Basic)
		if _, err := service.QuoteChange(ctx, "alice", plan.Basic, TermMonthly); !errors.Is(err, ErrNoChange) {
			t.Fatalf("same selection = %v", err)
		}
		// BILL-6, BILL-19: a tier and term change together waits for the paid term end.
		quote, err := service.QuoteChange(ctx, "alice", plan.Pro, TermAnnual)
		if err != nil || quote.AppliedNow || !quote.EffectiveAt.Equal(termEnd) {
			t.Fatalf("tier plus term = %+v, err=%v", quote, err)
		}
		store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, TermAnnual, anchor, time.Date(2027, 1, 15, 0, 0, 0, 0, seoul), termEnd, true)
		quote, err = service.QuoteChange(ctx, "alice", plan.Basic, TermMonthly)
		if err != nil || quote.AppliedNow {
			t.Fatalf("annual to monthly = %+v, err=%v", quote, err)
		}
	})

	// BILL-5: a monthly upgrade charges the current tier's prorated difference, and a repeated
	// upgrade prices from the tier it reached rather than the original one.
	t.Run("monthly upgrades charge the current tier difference and raise the open lot", func(t *testing.T) {
		store := newSubscriptionStore()
		subscription := monthly(plan.Basic)
		scheduled := plan.Light
		scheduledTerm := TermMonthly
		subscription.ScheduledTier, subscription.ScheduledTerm = &scheduled, &scheduledTerm
		store.subscriptions["alice"] = subscription
		provider := newSubscriptionProvider()
		service := newSubscriptionService(store, provider, now)

		wantKRW, _ := plan.ProrateCeil(9900-4900, termStart, termEnd, now)
		quote, updated, appliedNow, err := quoteAndChange(ctx, t, service, plan.Pro, TermMonthly)
		if err != nil {
			t.Fatal(err)
		}
		if !quote.AppliedNow || quote.KRW != int(wantKRW) || !quote.EffectiveAt.Equal(now) {
			t.Fatalf("quote = %+v, want %d KRW now", quote, wantKRW)
		}
		if !appliedNow || updated.Tier != plan.Pro || updated.ScheduledTier != nil || updated.ScheduledTerm != nil {
			t.Fatalf("updated = %+v applied=%v", updated, appliedNow)
		}
		if len(provider.requests) != 1 || provider.requests[0].KRW != quote.KRW {
			t.Fatalf("requests = %+v", provider.requests)
		}
		benefitStart, benefitEnd := plan.BenefitWindow(anchor, now)
		wantBonus, _ := plan.ProrateCeil(1070-510, benefitStart, benefitEnd, now)
		if kinds(store.events) != "charge,tier_change" || len(store.credits.raises) != 1 || store.credits.raises[0] != int(wantBonus) || store.plans.tiers["alice"] != plan.Pro {
			t.Fatalf("events=%s raises=%v tier=%s", kinds(store.events), store.credits.raises, store.plans.tiers["alice"])
		}

		later := now.Add(time.Second)
		service.now = func() time.Time { return later }
		if _, _, _, err := quoteAndChange(ctx, t, service, plan.Max, TermMonthly); err != nil {
			t.Fatal(err)
		}
		wantSecond, _ := plan.ProrateCeil(29900-9900, termStart, termEnd, later)
		if got := *store.events[2].KRW; store.events[2].Kind != "charge" || got != int(wantSecond) {
			t.Fatalf("second upgrade = %+v, want %d KRW", store.events[2], wantSecond)
		}
	})

	t.Run("provider failure leaves subscription and credit state untouched", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = monthly(plan.Basic)
		provider := newSubscriptionProvider()
		provider.chargeErr = declined
		service := newSubscriptionService(store, provider, now)
		if _, _, _, err := quoteAndChange(ctx, t, service, plan.Pro, TermMonthly); !errors.Is(err, ErrChargeFailed) {
			t.Fatalf("error = %v", err)
		}
		if store.subscriptions["alice"].Tier != plan.Basic || len(store.credits.raises) != 0 || store.plans.tiers["alice"] != plan.Free || kinds(store.events) != "charge_failed" {
			t.Fatalf("subscription=%+v raises=%v tier=%s events=%s", store.subscriptions["alice"], store.credits.raises, store.plans.tiers["alice"], kinds(store.events))
		}
	})

	t.Run("the next pass applies a provider charge whose response was lost, once", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = monthly(plan.Basic)
		provider := newSubscriptionProvider()
		provider.failAfterCharge = true
		service := newSubscriptionService(store, provider, now)
		if _, _, _, err := quoteAndChange(ctx, t, service, plan.Pro, TermMonthly); !errors.Is(err, ErrPaymentPending) {
			t.Fatalf("first change = %v", err)
		}
		service.now = func() time.Time { return now.Add(pendingSettleGrace) }
		if err := service.ReconcilePending(ctx); err != nil {
			t.Fatalf("pass = %v", err)
		}
		if len(provider.requests) != 1 || store.subscriptions["alice"].Tier != plan.Pro || len(store.credits.raises) != 1 {
			t.Fatalf("requests=%d subscription=%+v raises=%v", len(provider.requests), store.subscriptions["alice"], store.credits.raises)
		}
	})
}

// An upgrade charges the quoted proration of the paid term, and the card is asked for exactly
// the quoted amount (BILL-5, BILL-18): on an annual term's first day that is the whole annual
// difference.
func TestUpgradeChargesTheQuotedProrationOfThePaidTerm(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	annualEnd := time.Date(2027, 1, 15, 0, 0, 0, 0, seoul)
	monthlyEnd := time.Date(2026, 2, 15, 0, 0, 0, 0, seoul)
	basic, _ := plan.CommercialOffer(plan.Basic)
	pro, _ := plan.CommercialOffer(plan.Pro)
	annualDifference := int64(pro.AnnualKRW - basic.AnnualKRW)
	midTerm := time.Date(2026, 6, 14, 23, 0, 0, 0, seoul)
	midTermKRW, _ := plan.ProrateCeil(annualDifference, anchor, annualEnd, midTerm)
	monthlyAt := time.Date(2026, 1, 20, 0, 0, 0, 0, seoul)
	monthlyKRW, _ := plan.ProrateCeil(int64(pro.MonthlyKRW-basic.MonthlyKRW), anchor, monthlyEnd, monthlyAt)

	for _, test := range []struct {
		name    string
		term    Term
		termEnd time.Time
		now     time.Time
		wantKRW int64
	}{
		{"the annual term's first day is charged the whole annual difference", TermAnnual, annualEnd, anchor, annualDifference},
		{"mid-term charges the remaining share of the annual difference", TermAnnual, annualEnd, midTerm, midTermKRW},
		{"a monthly term charges the remaining share of the monthly difference", TermMonthly, monthlyEnd, monthlyAt, monthlyKRW},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newSubscriptionStore()
			store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, test.term, anchor, test.termEnd, test.termEnd, true)
			provider := newSubscriptionProvider()
			service := newSubscriptionService(store, provider, test.now)

			quote, _, applied, err := quoteAndChange(ctx, t, service, plan.Pro, test.term)
			if err != nil || !applied || !quote.AppliedNow || quote.KRW != int(test.wantKRW) {
				t.Fatalf("quote=%+v applied=%v err=%v, want %d KRW", quote, applied, err, test.wantKRW)
			}
			// The quote and the card must never disagree: the charge is what the provider
			// was actually asked for.
			if len(provider.requests) != 1 || provider.requests[0].KRW != quote.KRW {
				t.Fatalf("charge requests = %+v, quote KRW = %d", provider.requests, quote.KRW)
			}
			if kinds(store.events) != "charge,tier_change" {
				t.Fatalf("events = %s", kinds(store.events))
			}
		})
	}
}

func TestScheduledChangesReplaceCancelAndApplyAtTermEnd(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	now := time.Date(2026, 1, 20, 0, 0, 0, 0, seoul)
	termEnd := time.Date(2026, 2, 15, 0, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermMonthly, anchor, termEnd, termEnd, false)
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)
	basic, _ := plan.CommercialOffer(plan.Basic)

	quote, updated, applied, err := quoteAndChange(ctx, t, service, plan.Basic, TermMonthly)
	if err != nil || quote.AppliedNow || !quote.EffectiveAt.Equal(termEnd) || quote.KRW != basic.MonthlyKRW {
		t.Fatalf("scheduled quote=%+v err=%v", quote, err)
	}
	if applied || !updated.AutoRenew || *updated.ScheduledTier != plan.Basic || kinds(store.events) != "change_scheduled" {
		t.Fatalf("updated=%+v applied=%v events=%s", updated, applied, kinds(store.events))
	}
	_, updated, applied, err = quoteAndChange(ctx, t, service, plan.Pro, TermAnnual)
	if err != nil || applied || *updated.ScheduledTier != plan.Pro || *updated.ScheduledTerm != TermAnnual || kinds(store.events) != "change_scheduled,change_scheduled" {
		t.Fatalf("replacement=%+v applied=%v events=%s err=%v", updated, applied, kinds(store.events), err)
	}
	if _, err := service.CancelScheduledChange(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CancelScheduledChange(ctx, "alice"); !errors.Is(err, ErrNoScheduledChange) {
		t.Fatalf("second cancel = %v", err)
	}

	if _, _, _, err := quoteAndChange(ctx, t, service, plan.Basic, TermMonthly); err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 0 {
		t.Fatalf("scheduling charged: %+v", provider.requests)
	}
	service.now = func() time.Time { return termEnd }
	if err := service.RunDue(ctx, termEnd); err != nil {
		t.Fatal(err)
	}
	landed := store.subscriptions["alice"]
	if landed.Tier != plan.Basic || landed.Term != TermMonthly || landed.ScheduledTier != nil || landed.ScheduledTerm != nil || store.plans.tiers["alice"] != plan.Basic {
		t.Fatalf("landed=%+v assigned=%s", landed, store.plans.tiers["alice"])
	}
	if len(provider.requests) != 1 || provider.requests[0].KRW != basic.MonthlyKRW || kinds(store.events) != "change_scheduled,change_scheduled,change_scheduled,charge,tier_change" {
		t.Fatalf("requests=%+v events=%s", provider.requests, kinds(store.events))
	}
}

func TestCancelAndResumeBeforeTermEnd(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	termEnd := time.Date(2026, 2, 15, 0, 0, 0, 0, seoul)
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	subscription := activeSubscription("alice", plan.Pro, TermMonthly, anchor, termEnd, termEnd, true)
	scheduled := plan.Basic
	scheduledTerm := TermMonthly
	subscription.ScheduledTier, subscription.ScheduledTerm = &scheduled, &scheduledTerm
	store.subscriptions["alice"] = subscription
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)

	cancelled, err := service.CancelSubscription(ctx, "alice")
	if err != nil || cancelled.AutoRenew || cancelled.ScheduledTier != nil || kinds(store.events) != "cancel_scheduled" {
		t.Fatalf("cancelled=%+v events=%s err=%v", cancelled, kinds(store.events), err)
	}
	resumed, err := service.ResumeSubscription(ctx, "alice")
	if err != nil || !resumed.AutoRenew {
		t.Fatalf("resumed=%+v err=%v", resumed, err)
	}
	service.now = func() time.Time { return termEnd }
	if _, err := service.ResumeSubscription(ctx, "alice"); !errors.Is(err, ErrSubscriptionRequired) {
		t.Fatalf("late resume = %v", err)
	}
	if err := service.RunDue(ctx, termEnd); err != nil {
		t.Fatal(err)
	}
	if store.subscriptions["alice"].Status != "active" || len(provider.requests) != 1 {
		t.Fatalf("subscription=%+v requests=%+v", store.subscriptions["alice"], provider.requests)
	}
}
