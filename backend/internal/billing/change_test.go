package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

func TestChangeSubscriptionClassifiesAndChargesOnlyUpgrades(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, seoul)
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	termEnd := time.Date(2027, 1, 15, 0, 0, 0, 0, seoul)

	t.Run("refusals use the shared classifier", func(t *testing.T) {
		store := newSubscriptionStore()
		service := newSubscriptionService(store, newSubscriptionProvider(), now)
		if _, err := service.QuoteChange(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrSubscriptionRequired) {
			t.Fatalf("missing subscription = %v", err)
		}
		store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, TermMonthly, anchor, termEnd, termEnd, true)
		if _, _, err := service.ChangeSubscription(ctx, "alice", plan.Basic, TermMonthly); !errors.Is(err, ErrNoChange) {
			t.Fatalf("same selection = %v", err)
		}
		if _, err := service.QuoteChange(ctx, "alice", plan.Pro, TermAnnual); !errors.Is(err, ErrChangeUnsupported) {
			t.Fatalf("tier plus term = %v", err)
		}
		store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, TermAnnual, anchor, termEnd, termEnd, true)
		quote, err := service.QuoteChange(ctx, "alice", plan.Basic, TermMonthly)
		if err != nil || quote.AppliedNow {
			t.Fatalf("annual to monthly = %+v, err=%v", quote, err)
		}
	})

	t.Run("monthly upgrades charge the current tier difference and raise the open lot", func(t *testing.T) {
		store := newSubscriptionStore()
		subscription := activeSubscription("alice", plan.Basic, TermMonthly, anchor, termEnd, termEnd, true)
		scheduled := plan.Free
		scheduledTerm := TermMonthly
		subscription.ScheduledTier, subscription.ScheduledTerm = &scheduled, &scheduledTerm
		store.subscriptions["alice"] = subscription
		provider := newSubscriptionProvider()
		service := newSubscriptionService(store, provider, now)

		quote, err := service.QuoteChange(ctx, "alice", plan.Pro, TermMonthly)
		if err != nil || !quote.AppliedNow || quote.USDCents != 300 || !quote.EffectiveAt.Equal(now) {
			t.Fatalf("quote = %+v, err=%v", quote, err)
		}
		updated, appliedNow, err := service.ChangeSubscription(ctx, "alice", plan.Pro, TermMonthly)
		if err != nil {
			t.Fatal(err)
		}
		if !appliedNow || updated.Tier != plan.Pro || updated.ScheduledTier != nil || updated.ScheduledTerm != nil {
			t.Fatalf("updated = %+v applied=%v", updated, appliedNow)
		}
		if len(provider.requests) != 1 || provider.requests[0].OrderID != "upg:alice:2026-06-14T03:00:00Z" || provider.requests[0].KRW != 4_178 {
			t.Fatalf("requests = %+v", provider.requests)
		}
		if kinds(store.events) != "charge,tier_change" || len(store.credits.raises) != 1 || store.credits.raises[0] != 355 || store.plans.tiers["alice"] != plan.Pro {
			t.Fatalf("events=%s raises=%v tier=%s", kinds(store.events), store.credits.raises, store.plans.tiers["alice"])
		}

		service.now = func() time.Time { return now.Add(time.Second) }
		if _, _, err := service.ChangeSubscription(ctx, "alice", plan.Max, TermMonthly); err != nil {
			t.Fatal(err)
		}
		if got := *store.events[2].USDCents; got != 500 {
			t.Fatalf("second upgrade cents = %d", got)
		}
	})

	t.Run("provider failure leaves subscription and credit state untouched", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, TermMonthly, anchor, termEnd, termEnd, true)
		provider := newSubscriptionProvider()
		provider.chargeErr = errors.New("declined")
		service := newSubscriptionService(store, provider, now)
		if _, _, err := service.ChangeSubscription(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrChargeFailed) {
			t.Fatalf("error = %v", err)
		}
		if store.subscriptions["alice"].Tier != plan.Basic || len(store.credits.raises) != 0 || store.plans.tiers["alice"] != plan.Free || kinds(store.events) != "charge_failed" {
			t.Fatalf("subscription=%+v raises=%v tier=%s events=%s", store.subscriptions["alice"], store.credits.raises, store.plans.tiers["alice"], kinds(store.events))
		}
	})

	t.Run("a retry recovers a provider charge whose response was lost", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, TermMonthly, anchor, termEnd, termEnd, true)
		provider := newSubscriptionProvider()
		provider.failAfterCharge = true
		service := newSubscriptionService(store, provider, now)
		if _, _, err := service.ChangeSubscription(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrChargeFailed) {
			t.Fatalf("first change = %v", err)
		}
		if _, _, err := service.ChangeSubscription(ctx, "alice", plan.Pro, TermMonthly); err != nil {
			t.Fatalf("retry = %v", err)
		}
		if len(provider.requests) != 1 || store.subscriptions["alice"].Tier != plan.Pro || len(store.credits.raises) != 1 {
			t.Fatalf("requests=%d subscription=%+v raises=%v", len(provider.requests), store.subscriptions["alice"], store.credits.raises)
		}
	})
}

// An upgrade's span counts the running month as whole and prices it at a twelfth of the
// annual price (BILLING-18): counting only completed windows made an upgrade inside the last
// month free and charged a fresh term for eleven of the twelve windows it grants.
func TestUpgradeChargesTheMonthsStillToRunAtTheTermsOwnUnitPrice(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	annualEnd := time.Date(2027, 1, 15, 0, 0, 0, 0, seoul)
	monthlyEnd := time.Date(2026, 2, 15, 0, 0, 0, 0, seoul)

	// basic 200c/mo, pro 500c/mo; annual is ten months for twelve, so 2000c and 5000c a year
	// and a monthly equivalent of 250c on the difference.
	annualDifference := PriceCents(plan.Pro, TermAnnual) - PriceCents(plan.Basic, TermAnnual)

	for _, test := range []struct {
		name      string
		term      Term
		termEnd   time.Time
		now       time.Time
		wantCents int
	}{
		{
			name: "the term's first day is charged for all twelve windows",
			term: TermAnnual, termEnd: annualEnd, now: anchor,
			wantCents: annualDifference,
		},
		{
			name: "mid-term counts the running month whole",
			term: TermAnnual, termEnd: annualEnd, now: time.Date(2026, 6, 14, 23, 0, 0, 0, seoul),
			wantCents: 2_000,
		},
		{
			name: "an anchor boundary belongs to the window it opens",
			term: TermAnnual, termEnd: annualEnd, now: time.Date(2026, 6, 15, 0, 0, 0, 0, seoul),
			wantCents: 1_750,
		},
		{
			name: "the last window is charged for one month, never nothing",
			term: TermAnnual, termEnd: annualEnd, now: time.Date(2026, 12, 15, 1, 0, 0, 0, seoul),
			wantCents: annualDifference / 12,
		},
		{
			name: "a monthly term is the full difference between the two monthly prices",
			term: TermMonthly, termEnd: monthlyEnd, now: time.Date(2026, 1, 20, 0, 0, 0, 0, seoul),
			wantCents: plan.MonthlyPriceCents(plan.Pro) - plan.MonthlyPriceCents(plan.Basic),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newSubscriptionStore()
			store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, test.term, anchor, test.termEnd, test.termEnd, true)
			provider := newSubscriptionProvider()
			service := newSubscriptionService(store, provider, test.now)

			quote, err := service.QuoteChange(ctx, "alice", plan.Pro, test.term)
			if err != nil || quote.USDCents != test.wantCents || !quote.AppliedNow {
				t.Fatalf("quote=%+v err=%v, want %d cents", quote, err, test.wantCents)
			}
			if _, applied, err := service.ChangeSubscription(ctx, "alice", plan.Pro, test.term); err != nil || !applied {
				t.Fatalf("applied=%v err=%v", applied, err)
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

// The identity that makes the rule self-consistent: twelve windows at a twelfth of the
// annual difference is the annual difference, with nothing lost to integer division.
func TestAnnualUpgradeOnTheFirstDayCostsTheAnnualDifference(t *testing.T) {
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	termEnd := time.Date(2027, 1, 15, 0, 0, 0, 0, seoul)
	if months := monthsStillToRun(anchor, termEnd, anchor); months != 12 {
		t.Fatalf("months still to run on the first day = %d, want 12", months)
	}
	for _, tier := range []plan.Plan{plan.Pro, plan.Max} {
		difference := PriceCents(tier, TermAnnual) - PriceCents(plan.Basic, TermAnnual)
		if difference*12/12 != difference {
			t.Fatalf("%s: %d cents does not survive the twelfth", tier, difference)
		}
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

	quote, err := service.QuoteChange(ctx, "alice", plan.Basic, TermMonthly)
	if err != nil || quote.AppliedNow || !quote.EffectiveAt.Equal(termEnd) || quote.USDCents != 200 {
		t.Fatalf("scheduled quote=%+v err=%v", quote, err)
	}
	updated, applied, err := service.ChangeSubscription(ctx, "alice", plan.Basic, TermMonthly)
	if err != nil || applied || !updated.AutoRenew || *updated.ScheduledTier != plan.Basic || kinds(store.events) != "change_scheduled" {
		t.Fatalf("updated=%+v applied=%v events=%s err=%v", updated, applied, kinds(store.events), err)
	}
	updated, applied, err = service.ChangeSubscription(ctx, "alice", plan.Pro, TermAnnual)
	if err != nil || applied || *updated.ScheduledTier != plan.Pro || *updated.ScheduledTerm != TermAnnual || kinds(store.events) != "change_scheduled,change_scheduled" {
		t.Fatalf("replacement=%+v applied=%v events=%s err=%v", updated, applied, kinds(store.events), err)
	}
	if _, err := service.CancelScheduledChange(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CancelScheduledChange(ctx, "alice"); !errors.Is(err, ErrNoScheduledChange) {
		t.Fatalf("second cancel = %v", err)
	}

	if _, _, err := service.ChangeSubscription(ctx, "alice", plan.Basic, TermMonthly); err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return termEnd }
	if err := service.RunDue(ctx, termEnd); err != nil {
		t.Fatal(err)
	}
	landed := store.subscriptions["alice"]
	if landed.Tier != plan.Basic || landed.Term != TermMonthly || landed.ScheduledTier != nil || landed.ScheduledTerm != nil || store.plans.tiers["alice"] != plan.Basic {
		t.Fatalf("landed=%+v assigned=%s", landed, store.plans.tiers["alice"])
	}
	if len(provider.requests) != 1 || provider.requests[0].KRW != 2_785 || kinds(store.events) != "change_scheduled,change_scheduled,change_scheduled,charge,tier_change" {
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
