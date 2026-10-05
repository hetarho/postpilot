package usage

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// fakeStore is an in-memory usage.Storage. These tests are about the hold rules, the
// consumption order and the settlement arithmetic, not about SQL — but the two guards the
// real statements carry (a lot never drops below zero, never rises above its grant) are
// reproduced here, because those are the invariant rather than an implementation detail.
type fakeStore struct {
	// postStageCosts is what PostStageCosts answers; postStageSince records the window asked for.
	postStageCosts []PostStageCost
	postStageSince time.Time
	lots           []Lot
	admissions     []Admission
	holdDebits     map[string][]LotDebit
	eligible       map[string][]string
	settled        map[string]int
	settlements    map[string]Settlement
	events         []Event
	lotSeq         int
	spendCalls     int
	raiseCalls     int
	writeTxs       int
	failOnSpend    error
}

func newFakeStore() *fakeStore {
	return &fakeStore{holdDebits: map[string][]LotDebit{}, eligible: map[string][]string{}, settled: map[string]int{}, settlements: map[string]Settlement{}}
}

// InWriteTx is a pass-through here: these tests assert the rules, and the real store's
// BEGIN IMMEDIATE is what makes them hold under concurrency. It counts the transactions.
func (f *fakeStore) InWriteTx(_ context.Context, fn func(Storage) error) error {
	f.writeTxs++
	return fn(f)
}

// The renewal reads apply the same predicates as the fake's writes below.
func (f *fakeStore) WindowGrantsOpened(_ context.Context, grants []Lot) (bool, error) {
	for _, grant := range grants {
		opened := false
		for _, lot := range f.lots {
			if lot.ID == grant.ID && lot.ExpiresAt != nil && grant.ExpiresAt != nil && !lot.ExpiresAt.Before(*grant.ExpiresAt) {
				opened = true
			}
		}
		if !opened {
			return false, nil
		}
	}
	return true, nil
}
func (f *fakeStore) ExportWindowOpened(context.Context, ExportWindow) (bool, error) { return true, nil }
func (f *fakeStore) ReadLotsInConsumptionOrder(ctx context.Context, userID string, now time.Time) ([]Lot, error) {
	return f.LotsInConsumptionOrder(ctx, userID, now)
}

func (f *fakeStore) LotsInConsumptionOrder(_ context.Context, userID string, now time.Time) ([]Lot, error) {
	var out []Lot
	for _, lot := range f.lots {
		if lot.UserID != userID || lot.Remaining <= 0 || lot.Expired(now) {
			continue
		}
		out = append(out, lot)
	}
	// every expiring lot by expiry, then never-expiring bonus, then purchased, then
	// creation — the ORDER BY the query spells out (QUOTA-12).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && lotBefore(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

func lotBefore(a, b Lot) bool {
	if rankA, rankB := consumptionRank(a), consumptionRank(b); rankA != rankB {
		return rankA < rankB
	}
	switch {
	case a.ExpiresAt == nil && b.ExpiresAt == nil:
		return a.CreatedAt.Before(b.CreatedAt)
	case a.ExpiresAt == nil:
		return false
	case b.ExpiresAt == nil:
		return true
	case a.ExpiresAt.Equal(*b.ExpiresAt):
		return a.CreatedAt.Before(b.CreatedAt)
	default:
		return a.ExpiresAt.Before(*b.ExpiresAt)
	}
}

func consumptionRank(lot Lot) int {
	switch {
	case lot.Kind == LotPurchased:
		return 2
	case lot.ExpiresAt == nil:
		return 1
	default:
		return 0
	}
}

func (f *fakeStore) ExpireVoucherLot(_ context.Context, lotID string, at time.Time) (bool, error) {
	for i := range f.lots {
		lot := &f.lots[i]
		if lot.ID == lotID && lot.Kind == LotVoucher && lot.ExpiresAt != nil && lot.ExpiresAt.After(at) {
			moved := at
			lot.ExpiresAt = &moved
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) VoucherLots(_ context.Context, lotIDs []string) ([]Lot, error) {
	wanted := map[string]bool{}
	for _, id := range lotIDs {
		wanted[id] = true
	}
	var out []Lot
	for _, lot := range f.lots {
		if wanted[lot.ID] && lot.Kind == LotVoucher {
			out = append(out, lot)
		}
	}
	return out, nil
}

func (f *fakeStore) ActiveMonthlyLot(_ context.Context, userID string, now time.Time) (Lot, bool, error) {
	for _, lot := range f.lots {
		if lot.UserID == userID && lot.Kind == LotMonthly && !lot.Expired(now) {
			return lot, true, nil
		}
	}
	return Lot{}, false, nil
}

func (f *fakeStore) InsertLot(_ context.Context, lot Lot) error {
	f.lotSeq++
	if lot.ID == "" {
		lot.ID = fmt.Sprintf("lot-%d", f.lotSeq)
	}
	f.lots = append(f.lots, lot)
	return nil
}

func (f *fakeStore) InsertLotIfAbsent(ctx context.Context, lot Lot) (bool, error) {
	for _, existing := range f.lots {
		if existing.ID == lot.ID {
			return false, nil
		}
	}
	return true, f.InsertLot(ctx, lot)
}

func (f *fakeStore) RaiseLot(_ context.Context, lotID string, credits int) error {
	f.raiseCalls++
	for i := range f.lots {
		if f.lots[i].ID == lotID {
			f.lots[i].Granted += credits
			f.lots[i].Remaining += credits
		}
	}
	return nil
}

func (f *fakeStore) UntouchedPurchasedLots(_ context.Context, lotIDs []string) ([]string, error) {
	wanted := map[string]bool{}
	for _, id := range lotIDs {
		wanted[id] = true
	}
	var untouched []string
	for _, lot := range f.lots {
		if wanted[lot.ID] && lot.Kind == LotPurchased && lot.Granted > 0 && lot.Remaining == lot.Granted {
			untouched = append(untouched, lot.ID)
		}
	}
	return untouched, nil
}

func (f *fakeStore) SpendFromLot(_ context.Context, lotID string, credits int) error {
	f.spendCalls++
	if f.failOnSpend != nil {
		return f.failOnSpend
	}
	for i := range f.lots {
		// The `remaining >= ?` guard the statement carries: a stale caller cannot drive a
		// lot negative.
		if f.lots[i].ID == lotID && f.lots[i].Remaining >= credits {
			f.lots[i].Remaining -= credits
		}
	}
	return nil
}

func (f *fakeStore) RefundToLot(_ context.Context, lotID string, credits int) error {
	for i := range f.lots {
		if f.lots[i].ID == lotID && f.lots[i].Remaining+credits <= f.lots[i].Granted {
			f.lots[i].Remaining += credits
		}
	}
	return nil
}

func (f *fakeStore) InsertAdmission(_ context.Context, admission Admission) error {
	f.admissions = append(f.admissions, admission)
	return nil
}

func (f *fakeStore) InsertHoldDebits(_ context.Context, jobID string, debits []LotDebit) error {
	if len(debits) > 0 {
		f.holdDebits[jobID] = append(f.holdDebits[jobID], debits...)
	}
	return nil
}

func (f *fakeStore) InsertEligibleLots(_ context.Context, jobID string, lots []Lot) error {
	for _, lot := range lots {
		f.eligible[jobID] = append(f.eligible[jobID], lot.ID)
	}
	return nil
}
func (f *fakeStore) EligibleLotsForJob(_ context.Context, jobID string) ([]Lot, error) {
	wanted := map[string]bool{}
	for _, id := range f.eligible[jobID] {
		wanted[id] = true
	}
	var lots []Lot
	for _, lot := range f.lots {
		if wanted[lot.ID] && lot.Remaining > 0 {
			lots = append(lots, lot)
		}
	}
	return lots, nil
}
func (f *fakeStore) DeleteEligibleLotsForJob(_ context.Context, jobID string) error {
	delete(f.eligible, jobID)
	return nil
}
func (f *fakeStore) OpenExportWindow(context.Context, ExportWindow) error { return nil }
func (f *fakeStore) HoldForJob(_ context.Context, jobID string) (Admission, []LotDebit, bool, error) {
	for _, admission := range f.admissions {
		if admission.JobID != jobID {
			continue
		}
		if _, done := f.settled[jobID]; done {
			return Admission{}, nil, false, nil
		}
		return admission, f.holdDebits[jobID], true, nil
	}
	return Admission{}, nil, false, nil
}

func (f *fakeStore) MarkSettled(_ context.Context, jobID string, settlement Settlement, _ time.Time) error {
	f.settled[jobID] = settlement.Credits
	f.settlements[jobID] = settlement
	return nil
}

func (f *fakeStore) UnsettledHoldJobs(_ context.Context) ([]string, error) {
	var out []string
	for _, admission := range f.admissions {
		if _, done := f.settled[admission.JobID]; !done {
			out = append(out, admission.JobID)
		}
	}
	return out, nil
}

func (f *fakeStore) DeleteAdmissionForJob(_ context.Context, jobID string) error {
	kept := f.admissions[:0]
	for _, admission := range f.admissions {
		if admission.JobID != jobID {
			kept = append(kept, admission)
		}
	}
	f.admissions = kept
	delete(f.holdDebits, jobID)
	return nil
}

func (f *fakeStore) InsertEvent(_ context.Context, event Event) error {
	f.events = append(f.events, event)
	return nil
}

func (f *fakeStore) PostStageCosts(_ context.Context, since time.Time) ([]PostStageCost, error) {
	f.postStageSince = since
	return f.postStageCosts, nil
}

func (f *fakeStore) ReasoningSpend(_ context.Context, stage string, since time.Time) ([]ReasoningSpend, error) {
	byModel := map[string]*ReasoningSpend{}
	for _, event := range f.events {
		if event.Stage != stage || event.CreatedAt.Before(since) {
			continue
		}
		row, ok := byModel[event.Model]
		if !ok {
			row = &ReasoningSpend{Model: event.Model, Stage: stage}
			byModel[event.Model] = row
		}
		row.Calls++
		row.ReasoningTokens += event.ReasoningTokens
		row.CompletionTokens += event.CompletionTokens
		if event.ReasoningTruncated {
			row.ReasoningTruncations++
		}
	}
	out := make([]ReasoningSpend, 0, len(byModel))
	for _, row := range byModel {
		out = append(out, *row)
	}
	return out, nil
}

func (f *fakeStore) CostForJob(_ context.Context, jobID string) (JobCost, error) {
	var cost JobCost
	for _, event := range f.events {
		if event.JobID == jobID {
			cost.TotalMicrousd += event.CostMicrousd
			if event.CostMicrousd > 0 && (event.CostSource == llm.CostReported || event.CostSource == llm.CostEstimated) {
				cost.ConfirmedMicrousd += event.CostMicrousd
			}
		}
	}
	return cost, nil
}

func (f *fakeStore) balance(userID string, now time.Time) int {
	total := 0
	for _, lot := range f.lots {
		if lot.UserID == userID && !lot.Expired(now) {
			total += lot.Remaining
		}
	}
	return total
}

type fakeModels map[llm.ModelRef]llm.ModelInfo

func (m fakeModels) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	info, ok := m[ref]
	return info, ok
}

type fakeAnchors struct {
	anchor time.Time
	err    error
}

func (a fakeAnchors) AnchorFor(context.Context, string) (time.Time, error) {
	return a.anchor, a.err
}
func (a fakeAnchors) CoverageFor(_ context.Context, _ string, at time.Time) (Coverage, bool, error) {
	if a.err != nil {
		return Coverage{}, false, a.err
	}
	return Coverage{ID: "test-coverage", Anchor: a.anchor,
		Tier: plan.Basic, DailyTier: plan.Basic}, true, nil
}

// seoulNoon is a fixed instant inside one Asia/Seoul month, far from either boundary.
var seoulNoon = time.Date(2026, 9, 15, 3, 0, 0, 0, time.UTC) // 15 Sep, 12:00 KST
var testAnchor = time.Date(2025, 1, 1, 0, 0, 0, 0, time.FixedZone("Asia/Seoul", 9*60*60))

var cheapRef = llm.ModelRef{ProviderID: "openrouter", ModelID: "cheap"}

// The hold prices 30 000 prompt tokens plus the completion cap; these rates make that
// exactly 10 000 micro-USD, which converts to 5 credits at testReferenceE4.
var pricedModels = fakeModels{
	cheapRef: {Ref: cheapRef, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"},
}

const maxCompletion = 10_000

// testReferenceE4 is 500 KRW per USD, so the 10 000 micro-USD one cheap call holds converts to
// exactly 5 credits.
const testReferenceE4 = 5_000_000

// testRates is the ledger's FX policy at testReferenceE4.
func testRates() *RateSelector { return NewFixedRateSelector(testReferenceE4) }

func newTestService(t *testing.T, now time.Time) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	// The two kinds the product marks as needing an approved ceiling; the ledger only
	// knows them because the root says so.
	svc := NewService(store, pricedModels, maxCompletion, fakeAnchors{anchor: testAnchor}, testRates(), "generate_clip", "revise_clip")
	svc.now = func() time.Time { return now }
	seq := 0
	svc.newID = func() string { seq++; return fmt.Sprintf("lot-new-%d", seq) }
	return svc, store
}

// openMonthly is a monthly lot already open for the current month, so a test about the
// hold itself does not also trigger the renewal that a first access performs.
func openMonthly(userID string, remaining int) Lot {
	end := plan.NextRenewal(testAnchor, seoulNoon)
	return Lot{
		ID: "monthly-open", UserID: userID, Kind: LotMonthly,
		Granted: remaining, Remaining: remaining, ExpiresAt: &end,
	}
}

func TestBillingCreditOperationsPreserveLotInvariants(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	ctx := context.Background()

	purchasedID, err := svc.OpenPurchasedLot(ctx, "alice", 100)
	if err != nil {
		t.Fatalf("OpenPurchasedLot: %v", err)
	}
	purchased := store.lots[0]
	if purchased.ID != purchasedID || !strings.HasPrefix(purchasedID, "purchased:") || purchased.Kind != LotPurchased || purchased.ExpiresAt != nil || purchased.Granted != 100 || purchased.Remaining != 100 {
		t.Fatalf("purchased lot = %+v", purchased)
	}
	if created, err := svc.GrantBonusOnce(ctx, "payment-method:alice", "alice", 100); err != nil || !created {
		t.Fatalf("GrantBonusOnce: %v", err)
	}
	if created, err := svc.GrantBonusOnce(ctx, "payment-method:alice", "alice", 100); err != nil || created {
		t.Fatalf("GrantBonusOnce retry: %v", err)
	}
	if got := len(store.lots); got != 2 {
		t.Fatalf("lot count after idempotent bonus = %d, want 2", got)
	}
	bonus := store.lots[1]
	if bonus.Kind != LotBonus || bonus.ExpiresAt != nil || bonus.Granted != 100 || bonus.Remaining != 100 {
		t.Fatalf("bonus lot = %+v", bonus)
	}
}

func holdStart(userID string, tier plan.Plan, jobID string, calls ...PlannedCall) Start {
	return Start{UserID: userID, Plan: tier, Kind: "generate", JobID: jobID, Calls: calls}
}

// The hold's assumed shape prices one cheap call at 10 000 micro-USD: 5 credits at 500 KRW
// per USD.
const oneCallHold = 5

func TestHoldChargesTheWorstCaseAndRecordsIt(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{
		openMonthly("alice", 0),
		{ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100},
	}

	if err := svc.Hold(context.Background(), holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 1})); err != nil {
		t.Fatal(err)
	}

	if got := store.admissions[0].HoldCredits; got != oneCallHold {
		t.Errorf("hold = %d, want %d", got, oneCallHold)
	}
	if got, want := store.balance("alice", seoulNoon), 100-oneCallHold; got != want {
		t.Errorf("balance = %d, want %d", got, want)
	}
	if got := store.holdDebits["job-1"]; len(got) != 1 || got[0].LotID != "bonus" || got[0].Credits != oneCallHold {
		t.Errorf("hold debits = %+v", got)
	}
}

// The count is what separates a job that observes once from one that observes eight
// times; pricing the ref instead of the calls would hold a fraction of the real cost.
func TestHoldPricesEveryPlannedCall(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 500, Remaining: 500}}

	if err := svc.Hold(context.Background(), holdStart("alice", plan.Max, "job-1", PlannedCall{Ref: cheapRef, Count: 4})); err != nil {
		t.Fatal(err)
	}
	// Four calls of 10 000 micro-USD, converted once at 500 KRW per USD.
	if got, want := store.admissions[0].HoldCredits, 4*oneCallHold; got != want {
		t.Errorf("hold = %d, want %d", got, want)
	}
}

func TestHoldAndQuotePriceEachCallsOwnCompletionBudget(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 500, Remaining: 500}}
	short := []PlannedCall{
		{Ref: cheapRef, Count: 2, CompletionTokens: 1024},
		{Ref: cheapRef, Count: 1, CompletionTokens: maxCompletion},
	}
	long := []PlannedCall{
		{Ref: cheapRef, Count: 2, CompletionTokens: 1024},
		{Ref: cheapRef, Count: 1, CompletionTokens: 30_000},
	}
	shortQuote := svc.CreditsFor(short)
	longQuote := svc.CreditsFor(long)
	if longQuote <= shortQuote {
		t.Fatalf("long quote = %d, short quote = %d; the write budget did not change the hold", longQuote, shortQuote)
	}
	if err := svc.Hold(context.Background(), holdStart("alice", plan.Max, "job-1", short...)); err != nil {
		t.Fatal(err)
	}
	if got := store.admissions[0].HoldCredits; got != shortQuote {
		t.Errorf("hold = %d, CreditsFor = %d for identical planned work", got, shortQuote)
	}
	// A caller that has not adopted per-stage budgets yet still receives the registry
	// default rather than accidentally pricing its completion at zero.
	if got, want := svc.CreditsFor([]PlannedCall{{Ref: cheapRef, Count: 1}}), svc.CreditsFor([]PlannedCall{{Ref: cheapRef, Count: 1, CompletionTokens: maxCompletion}}); got != want {
		t.Errorf("fallback quote = %d, explicit default quote = %d", got, want)
	}
}

// QUOTA-14: a call that declares its prompt is priced at the larger of that prompt and the
// 30 000-token default, so a small declaration holds what an undeclared call holds and a large
// one is refused when the balance covers only the default.
func TestHoldPricesTheLargerOfTheDeclaredPromptAndTheDefault(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	// cheap is 0.1 USD per million prompt tokens and 0.7 per million completion tokens.
	for _, tc := range []struct {
		prompt int64
		want   int64
	}{{0, 3_000 + 7_000}, {12_000, 3_000 + 7_000}, {holdInputTokens, 3_000 + 7_000}, {50_000, 5_000 + 7_000}} {
		got, err := svc.worstCaseMicrousd([]PlannedCall{{Ref: cheapRef, Count: 1, PromptTokens: tc.prompt}})
		if err != nil || got != tc.want {
			t.Fatalf("prompt %d priced %d µUSD (err %v), want %d", tc.prompt, got, err, tc.want)
		}
	}

	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: oneCallHold, Remaining: oneCallHold}}
	err := svc.Hold(context.Background(), holdStart("alice", plan.Free, "job-large", PlannedCall{Ref: cheapRef, Count: 1, PromptTokens: 50_000}))
	var refusal *plan.InsufficientCreditsError
	if !errors.As(err, &refusal) || refusal.Required <= oneCallHold || len(store.admissions) != 0 {
		t.Fatalf("a large prompt over the balance = %v, admissions %+v", err, store.admissions)
	}
	if err := svc.Hold(context.Background(), holdStart("alice", plan.Free, "job-small", PlannedCall{Ref: cheapRef, Count: 1, PromptTokens: 12_000})); err != nil {
		t.Fatal(err)
	}
	if got := store.admissions[0].HoldCredits; got != oneCallHold {
		t.Errorf("a small declared prompt held %d, want the default %d", got, oneCallHold)
	}
}

func TestHoldRefusesWhatTheBalanceCannotCoverAndWritesNothing(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 3, Remaining: 3}}

	err := svc.Hold(context.Background(), holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 1}))

	var refusal *plan.InsufficientCreditsError
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want an insufficient-credits refusal", err)
	}
	if refusal.Required != oneCallHold || refusal.Balance != 3 {
		t.Errorf("refusal = %+v, want required %d and balance 3", refusal, oneCallHold)
	}
	if !refusal.RenewsAt.IsZero() {
		t.Error("a free account must have no credit renewal clock")
	}
	if len(store.admissions) != 0 {
		t.Errorf("admissions = %+v, want none after a refusal", store.admissions)
	}
	if got := store.balance("alice", seoulNoon); got != 3 {
		t.Errorf("balance = %d, want the untouched 3", got)
	}
}

// The monthly grant always carries the earlier expiry, so "spend it before the bonus"
// needs no rule of its own — it falls out of the one ordering.
func TestConsumptionSpendsTheSoonestExpiryFirst(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	monthEnd := plan.NextRenewal(testAnchor, seoulNoon)
	later := monthEnd.AddDate(0, 2, 0)
	store.lots = []Lot{
		{ID: "no-expiry", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100},
		{ID: "later-bonus", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100, ExpiresAt: &later},
		{ID: "monthly", UserID: "alice", Kind: LotMonthly, Granted: 4, Remaining: 4, ExpiresAt: &monthEnd},
	}

	if err := svc.Hold(context.Background(), holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 1})); err != nil {
		t.Fatal(err)
	}

	debits := store.holdDebits["job-1"]
	if len(debits) != 2 {
		t.Fatalf("debits = %+v, want the monthly lot drained then the next expiry", debits)
	}
	if debits[0].LotID != "monthly" || debits[0].Credits != 4 {
		t.Errorf("first debit = %+v, want the whole monthly lot", debits[0])
	}
	if debits[1].LotID != "later-bonus" || debits[1].Credits != 1 {
		t.Errorf("second debit = %+v, want the expiring bonus before the permanent one", debits[1])
	}
}

func TestExpiredLotsAreNotSpendable(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	lapsed := seoulNoon.Add(-time.Hour)
	store.lots = []Lot{
		openMonthly("alice", 0),
		{ID: "lapsed", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100, ExpiresAt: &lapsed},
	}

	err := svc.Hold(context.Background(), holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 1}))

	var refusal *plan.InsufficientCreditsError
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a refusal: an expired lot is not a balance", err)
	}
}

func TestAccessOpensOnlyTheCurrentDailyAndMonthlyBenefits(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)

	balance, err := svc.BalanceFor(context.Background(), "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Credits != 45+510 || balance.DailyGrant != 45 || balance.MonthlyBonus != 510 {
		t.Errorf("balance = %+v, want separate basic daily and monthly benefits", balance)
	}
	_, dayEnd := plan.DailyWindow(testAnchor, seoulNoon)
	if !balance.RenewsAt.Equal(dayEnd) || !balance.DailyResetsAt.Equal(dayEnd) {
		t.Errorf("daily reset = %+v, want %s", balance, dayEnd)
	}
	if len(store.lots) != 2 {
		t.Fatalf("lots = %+v, want one daily and one monthly benefit", store.lots)
	}

	// A second read must not mint a second grant.
	if _, err := svc.BalanceFor(context.Background(), "alice", plan.Basic); err != nil {
		t.Fatal(err)
	}
	if len(store.lots) != 2 {
		t.Errorf("lots = %+v, want the same two", store.lots)
	}
}

// QUOTA-62, QUOTA-37: the export window is the anchored benefit month holding the instant —
// a boundary instant opens the next month, a day-31 anchor clamps to the month's end and
// returns to the 31st — ended early by a coverage end inside it, with the tier's allowance.
func TestExportWindowAtIsTheCappedBenefitMonth(t *testing.T) {
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)
	at := func(month time.Month, day, hour int) time.Time {
		return time.Date(2027, month, day, hour, 0, 0, 0, seoul)
	}
	anchor := at(time.January, 31, 10)
	open := Coverage{ID: "paid", Anchor: anchor, Tier: plan.Pro}
	for _, tc := range []struct {
		name       string
		coverage   Coverage
		at         time.Time
		start, end time.Time
	}{
		{"just before a boundary", open, at(time.February, 28, 10).Add(-time.Nanosecond), anchor, at(time.February, 28, 10)},
		{"at a boundary, clamped to February's end", open, at(time.February, 28, 10), at(time.February, 28, 10), at(time.March, 31, 10)},
		{"back on the 31st", open, at(time.April, 1, 0), at(time.March, 31, 10), at(time.April, 30, 10)},
		{"coverage ending inside the month", Coverage{ID: "paid", Anchor: anchor, End: at(time.February, 15, 0), Tier: plan.Pro},
			at(time.February, 10, 0), anchor, at(time.February, 15, 0)},
		{"coverage ending after the month", Coverage{ID: "paid", Anchor: anchor, End: at(time.June, 1, 0), Tier: plan.Pro},
			at(time.February, 10, 0), anchor, at(time.February, 28, 10)},
	} {
		window, ok := ExportWindowAt("alice", tc.coverage, tc.at)
		want := ExportWindow{UserID: "alice", CoverageID: "paid", Start: tc.start, End: tc.end, Allowance: 15}
		if !ok || !window.Start.Equal(want.Start) || !window.End.Equal(want.End) || window.UserID != want.UserID ||
			window.CoverageID != want.CoverageID || window.Allowance != want.Allowance {
			t.Errorf("%s: window = %+v ok=%v, want %+v", tc.name, window, ok, want)
		}
	}
	if _, ok := ExportWindowAt("alice", Coverage{ID: "op", Anchor: anchor, Tier: plan.Master}, anchor); ok {
		t.Error("a tier with no commercial offer produced an export window")
	}
}

func TestBenefitWindowsKeepExactAnchorAndIdempotentGrants(t *testing.T) {
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)
	anchor := time.Date(2025, 1, 20, 11, 0, 0, 0, seoul)
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, seoul)
	store := newFakeStore()
	svc := NewService(store, pricedModels, maxCompletion, fakeAnchors{anchor: anchor}, testRates())
	svc.now = func() time.Time { return now }

	first, err := svc.BalanceFor(context.Background(), "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.lots) != 2 || store.lots[0].Kind != LotDaily || store.lots[1].Kind != LotMonthly {
		t.Fatalf("first lots = %+v, want daily and monthly windows", store.lots)
	}
	if store.lots[0].Granted != 45 || store.lots[1].Granted != 510 ||
		store.lots[0].ExpiresAt == nil || !store.lots[0].ExpiresAt.Equal(first.RenewsAt) {
		t.Errorf("first lots = %+v, balance = %+v", store.lots, first)
	}
	if _, err := svc.BalanceFor(context.Background(), "alice", plan.Basic); err != nil {
		t.Fatal(err)
	}
	if len(store.lots) != 2 {
		t.Fatalf("second open in one window minted %d lots", len(store.lots))
	}

	now = first.RenewsAt
	second, err := svc.BalanceFor(context.Background(), "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.lots) != 3 || store.lots[2].Kind != LotDaily {
		t.Fatalf("next-window lots = %+v, want only the next daily grant", store.lots)
	}
	if !second.RenewsAt.Equal(first.RenewsAt.Add(24 * time.Hour)) {
		t.Errorf("next daily reset = %s, want 24 hours later", second.RenewsAt)
	}
}

func TestBalanceReadsTakeTheWriterOnlyWhenTheRenewalWouldWrite(t *testing.T) {
	now := seoulNoon
	store := newFakeStore()
	svc := NewService(store, pricedModels, maxCompletion, fakeAnchors{anchor: testAnchor}, testRates())
	svc.now = func() time.Time { return now }
	ctx := context.Background()

	first, err := svc.BalanceFor(ctx, "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.BalanceFor(ctx, "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	if store.writeTxs != 1 {
		t.Fatalf("two reads in one daily window opened %d write transactions, want 1", store.writeTxs)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("the read-pool answer %+v differs from the writer's %+v", second, first)
	}

	// The next daily window is a new grant; once it is open, reads are free again.
	now = first.RenewsAt
	for range 2 {
		if _, err := svc.BalanceFor(ctx, "alice", plan.Basic); err != nil {
			t.Fatal(err)
		}
	}
	if store.writeTxs != 2 {
		t.Fatalf("next window: %d write transactions, want 2", store.writeTxs)
	}

	// Free and master renew nothing, so their reads never take the writer.
	if _, err := svc.BalanceFor(ctx, "bob", plan.Free); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BalanceFor(ctx, "root", plan.Master); err != nil {
		t.Fatal(err)
	}
	if store.writeTxs != 2 {
		t.Fatalf("free and master reads opened %d write transactions", store.writeTxs-2)
	}
}

func TestLegacyFreeLotExpiresWithoutOpeningAnotherFreeGrant(t *testing.T) {
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)
	anchor := time.Date(2025, 1, 20, 0, 0, 0, 0, seoul)
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, seoul)
	calendarExpiry := time.Date(2026, 10, 1, 0, 0, 0, 0, seoul)
	store := newFakeStore()
	store.lots = []Lot{{
		ID: "legacy-calendar", UserID: "alice", Kind: LotMonthly,
		Granted: 50, Remaining: 50, ExpiresAt: &calendarExpiry,
	}}
	svc := NewService(store, pricedModels, maxCompletion, fakeAnchors{anchor: anchor}, testRates())
	svc.now = func() time.Time { return now }

	legacy, err := svc.BalanceFor(context.Background(), "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if !legacy.RenewsAt.IsZero() || legacy.Credits != 50 || len(store.lots) != 1 {
		t.Fatalf("legacy balance = %+v lots=%+v, want only the existing value", legacy, store.lots)
	}

	now = calendarExpiry
	transition, err := svc.BalanceFor(context.Background(), "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if !transition.RenewsAt.IsZero() || transition.Credits != 0 || len(store.lots) != 1 {
		t.Fatalf("transition = %+v lots=%+v, want no free renewal", transition, store.lots)
	}

	now = time.Date(2026, 10, 20, 0, 0, 0, 0, seoul)
	full, err := svc.BalanceFor(context.Background(), "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.lots) != 1 || full.Credits != 0 || !full.RenewsAt.IsZero() {
		t.Fatalf("full cycle = %+v lots=%+v", full, store.lots)
	}
}

func TestRenewalAfterTheBoundaryOpensTheNextGrant(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	lapsed := seoulNoon.Add(-time.Hour)
	store.lots = []Lot{{ID: "old", UserID: "alice", Kind: LotMonthly, Granted: 200, Remaining: 8, ExpiresAt: &lapsed}}

	balance, err := svc.BalanceFor(context.Background(), "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	// The lapsed lot's 8 remaining credits do not carry over.
	if balance.Credits != 45+510 {
		t.Errorf("credits = %d, want a fresh grant with nothing carried over", balance.Credits)
	}
	if len(store.lots) != 3 {
		t.Errorf("lots = %d, want the lapsed one kept and two current grants", len(store.lots))
	}
}

func TestMasterIsNeverRefusedButIsStillHeldAndRecorded(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	balance, err := svc.BalanceFor(context.Background(), "root", plan.Master)
	if err != nil {
		t.Fatal(err)
	}
	if !balance.Unlimited || !balance.RenewsAt.IsZero() {
		t.Fatalf("master balance = %+v, want unlimited without a credit reset", balance)
	}

	// No lots at all: an account that would be refused on any other tier.
	if err := svc.Hold(context.Background(), holdStart("root", plan.Master, "job-1", PlannedCall{Ref: cheapRef, Count: 3})); err != nil {
		t.Fatalf("master was refused: %v", err)
	}
	if len(store.admissions) != 1 {
		t.Fatalf("admissions = %+v, want the start recorded", store.admissions)
	}
	if store.admissions[0].HoldCredits == 0 {
		t.Error("master's hold was recorded as zero; its spend must stay readable")
	}
	if len(store.holdDebits["job-1"]) != 0 {
		t.Errorf("debits = %+v, want none: master spends no lot", store.holdDebits["job-1"])
	}
	if len(store.lots) != 0 {
		t.Errorf("lots = %+v, want none minted for an unlimited tier", store.lots)
	}
}

// An exempt tier holds nothing, so settlement must charge it nothing either — including
// when its work overran the estimate. A bonus lot granted to the operator is not a balance
// the gate ever consulted, and settlement must not become the path that drains it.
func TestSettleLeavesAnExemptTiersLotsAlone(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{{ID: "bonus", UserID: "root", Kind: LotBonus, Granted: 100, Remaining: 100}}
	ctx := context.Background()

	if err := svc.Hold(ctx, holdStart("root", plan.Master, "job-1", PlannedCall{Ref: cheapRef, Count: 1})); err != nil {
		t.Fatal(err)
	}
	// Far more than the recorded hold priced.
	store.events = append(store.events, Event{UserID: "root", JobID: "job-1", CostMicrousd: 5_000_000})

	if err := svc.Settle(ctx, "job-1", OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if got := store.balance("root", seoulNoon); got != 100 {
		t.Errorf("balance = %d, want the operator's lot untouched at 100", got)
	}
}

func TestSettleRefundsTheUnusedRemainderToTheSameLots(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100}}
	ctx := context.Background()

	if err := svc.Hold(ctx, holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 4})); err != nil {
		t.Fatal(err)
	}
	held := store.admissions[0].HoldCredits

	// The work actually cost 10 000 micro-USD: 5 credits at the frozen rate.
	store.events = append(store.events, Event{UserID: "alice", JobID: "job-1", CostMicrousd: 10_000, CostSource: llm.CostReported})

	if err := svc.Settle(ctx, "job-1", OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if got, want := store.settled["job-1"], 5; got != want {
		t.Errorf("settled = %d, want %d", got, want)
	}
	if got, want := store.balance("alice", seoulNoon), 100-5; got != want {
		t.Errorf("balance = %d, want %d (held %d, then refunded the difference)", got, want, held)
	}
}

func TestSettleIsIdempotent(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100}}
	ctx := context.Background()

	if err := svc.Hold(ctx, holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 4})); err != nil {
		t.Fatal(err)
	}
	store.events = append(store.events, Event{UserID: "alice", JobID: "job-1", CostMicrousd: 10_000, CostSource: llm.CostReported})

	if err := svc.Settle(ctx, "job-1", OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	after := store.balance("alice", seoulNoon)
	if err := svc.Settle(ctx, "job-1", OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if got := store.balance("alice", seoulNoon); got != after {
		t.Errorf("balance = %d after a second settle, want the unchanged %d", got, after)
	}
}

// The guarantee this whole change exists for: an estimate that came in low costs us the
// difference, never the account.
func TestSettleAboveTheHoldNeverDrivesTheBalanceNegative(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 6, Remaining: 6}}
	ctx := context.Background()

	if err := svc.Hold(ctx, holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 1})); err != nil {
		t.Fatal(err)
	}
	// Far more than the hold priced.
	store.events = append(store.events, Event{UserID: "alice", JobID: "job-1", CostMicrousd: 5_000_000, CostSource: llm.CostReported})

	if err := svc.Settle(ctx, "job-1", OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if got := store.balance("alice", seoulNoon); got < 0 {
		t.Fatalf("balance = %d, want never below zero", got)
	}
	if got := store.balance("alice", seoulNoon); got != 0 {
		t.Errorf("balance = %d, want the lots drained to exactly zero", got)
	}
}

func TestTwoConcurrentHoldsCannotBothPassOneBalance(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	// Enough for exactly one hold.
	store.lots = []Lot{
		openMonthly("alice", 0),
		{ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: oneCallHold, Remaining: oneCallHold},
	}
	ctx := context.Background()

	first := svc.Hold(ctx, holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 1}))
	second := svc.Hold(ctx, holdStart("alice", plan.Free, "job-2", PlannedCall{Ref: cheapRef, Count: 1}))

	if first != nil {
		t.Fatalf("the first hold was refused: %v", first)
	}
	var refusal *plan.InsufficientCreditsError
	if !errors.As(second, &refusal) {
		t.Fatalf("the second hold = %v, want a refusal", second)
	}
	if got := store.balance("alice", seoulNoon); got != 0 {
		t.Errorf("balance = %d, want zero rather than negative", got)
	}
}

func TestReleaseReturnsTheWholeHold(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100}}
	ctx := context.Background()

	if err := svc.Hold(ctx, holdStart("alice", plan.Free, "job-1", PlannedCall{Ref: cheapRef, Count: 4})); err != nil {
		t.Fatal(err)
	}
	if err := svc.Release(ctx, "job-1"); err != nil {
		t.Fatal(err)
	}
	if got := store.balance("alice", seoulNoon); got != 100 {
		t.Errorf("balance = %d, want the full 100 back", got)
	}
	if len(store.admissions) != 0 {
		t.Errorf("admissions = %+v, want the start removed", store.admissions)
	}
}

func TestConstructionRefusesALedgerWithoutAnchors(t *testing.T) {
	// The monthly-window resolver is a constructor argument (ARCH-40): a ledger that could
	// be built without one would silently anchor on a calendar month.
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "anchors") {
			t.Fatalf("panic = %v, want a loud anchor-wiring refusal", r)
		}
	}()
	NewService(newFakeStore(), pricedModels, maxCompletion, nil, testRates())
}

// QUOTA-59: every paid hold converts through the FX policy, so the selector is a constructor
// argument too (ARCH-40) and a ledger built without one refuses at construction.
func TestConstructionRefusesALedgerWithoutRates(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "rate selector") {
			t.Fatalf("panic = %v, want a loud rate-wiring refusal", r)
		}
	}()
	NewService(newFakeStore(), pricedModels, maxCompletion, fakeAnchors{anchor: testAnchor}, nil)
}

func TestRecordCallPricesAndAttributesFromContext(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "z-ai/glm-5.3-flash"}
	store := newFakeStore()
	svc := NewService(store, fakeModels{ref: {
		Ref: ref, InputUSDPerMillion: "0.075", OutputUSDPerMillion: "0.25",
	}}, maxCompletion, fakeAnchors{anchor: testAnchor}, testRates())
	svc.now = func() time.Time { return seoulNoon }
	ctx := WithWork(context.Background(), Work{
		UserID: "alice", Kind: "generate", JobID: "job", ObserveModel: "openrouter/observer",
	})

	if err := svc.RecordCall(ctx, ref, "", llm.Usage{PromptTokens: 1_000_000, CompletionTokens: 0}, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := len(store.events), 1; got != want {
		t.Fatalf("events = %d, want %d", got, want)
	}
	event := store.events[0]
	if event.CostMicrousd != 75_000 || event.CostSource != llm.CostEstimated {
		t.Errorf("cost = %d (%s), want 75000 estimated", event.CostMicrousd, event.CostSource)
	}
	if event.UserID != "alice" || event.JobID != "job" || event.Stage != "generate" {
		t.Errorf("attribution = %+v", event)
	}
}

func TestRecordCallKeepsFailedCallsWithReportedUsage(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	ctx := WithWork(context.Background(), Work{UserID: "alice", Kind: "revise", JobID: "job"})
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "gone"}

	// A failure that never reached a model has nothing to account for.
	if err := svc.RecordCall(ctx, ref, "", llm.Usage{}, errors.New("provider failed")); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %+v, want none for a failure with no usage", store.events)
	}

	// A failure the provider still billed is the case QUOTA-21 preserves usage for.
	if err := svc.RecordCall(ctx, ref, "", llm.Usage{PromptTokens: 12, CostMicrousd: 4, CostReported: true}, errors.New("provider failed")); err != nil {
		t.Fatal(err)
	}
	if got, want := len(store.events), 1; got != want {
		t.Fatalf("events = %d, want %d", got, want)
	}
	if store.events[0].CostSource != llm.CostReported || store.events[0].CostMicrousd != 4 {
		t.Errorf("event = %+v", store.events[0])
	}
}

// A10: the reasoning token count reaches the ledger row alongside the tokens and cost it
// already recorded, so "wrote 8,192 tokens of post" and "spent 8,192 tokens thinking and
// wrote nothing" stop being the same row.
func TestRecordCallStoresTheReasoningTokenCount(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	ctx := WithWork(context.Background(), Work{UserID: "alice", Kind: "generate", JobID: "job-1"})
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "writer"}

	if err := svc.RecordCall(ctx, ref, llm.StageNameWrite, llm.Usage{
		PromptTokens: 4304, CompletionTokens: 8192, ReasoningTokens: 8100,
	}, &llm.TruncatedError{ReasoningTokens: 8100, CompletionTokens: 8192}); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 1 {
		t.Fatalf("events = %d", len(store.events))
	}
	got := store.events[0]
	if got.ReasoningTokens != 8100 || got.CompletionTokens != 8192 || got.PromptTokens != 4304 {
		t.Fatalf("event = %+v", got)
	}
	if !got.ReasoningTruncated {
		t.Fatal("reasoning-caused truncation was not recorded")
	}
	// The stage the CALL named, not an inference from the ref.
	if got.Stage != llm.StageNameWrite {
		t.Errorf("stage = %q, want the stage the call named", got.Stage)
	}
	// A provider that reports no split leaves it zero, like every other usage field.
	if err := svc.RecordCall(ctx, ref, llm.StageNameWrite, llm.Usage{CompletionTokens: 40}, nil); err != nil {
		t.Fatal(err)
	}
	if store.events[1].ReasoningTokens != 0 {
		t.Errorf("an unreported split = %d, want 0", store.events[1].ReasoningTokens)
	}
}

// A11: the aggregate an operator reads is per model AND per stage, because the effort is per
// (model, purpose) — one averaged over the model would hide a writing stage that is spending
// its whole budget thinking behind an observation stage that is fine.
func TestReasoningSpendAggregatesPerModelAndStage(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	ctx := WithWork(context.Background(), Work{UserID: "alice", Kind: "generate", JobID: "job-1"})
	shared := llm.ModelRef{ProviderID: "openrouter", ModelID: "both-stages"}

	for _, call := range []struct {
		stage string
		usage llm.Usage
		err   error
	}{
		{llm.StageNameObserve, llm.Usage{CompletionTokens: 700, ReasoningTokens: 20}, nil},
		{llm.StageNameObserve, llm.Usage{CompletionTokens: 660, ReasoningTokens: 30}, nil},
		{llm.StageNameWrite, llm.Usage{CompletionTokens: 8192, ReasoningTokens: 8100}, &llm.TruncatedError{ReasoningTokens: 8100, CompletionTokens: 8192}},
	} {
		if err := svc.RecordCall(ctx, shared, call.stage, call.usage, call.err); err != nil {
			t.Fatal(err)
		}
	}

	observe, err := svc.ReasoningSpendByModel(context.Background(), llm.StageNameObserve)
	if err != nil {
		t.Fatal(err)
	}
	if len(observe) != 1 || observe[0].Calls != 2 || observe[0].ReasoningTokens != 50 || observe[0].CompletionTokens != 1360 {
		t.Fatalf("observe spend = %+v", observe)
	}
	write, err := svc.ReasoningSpendByModel(context.Background(), llm.StageNameWrite)
	if err != nil {
		t.Fatal(err)
	}
	if len(write) != 1 || write[0].Calls != 1 || write[0].ReasoningTokens != 8100 || write[0].ReasoningTruncations != 1 {
		t.Fatalf("write spend = %+v", write)
	}
	// The same model, the same window, opposite verdicts — which is the whole reason the
	// aggregate carries a stage.
	if float64(observe[0].ReasoningTokens)/float64(observe[0].CompletionTokens) >= 0.5 {
		t.Error("the observation stage reads as reasoning-heavy")
	}
	if float64(write[0].ReasoningTokens)/float64(write[0].CompletionTokens) <= 0.5 {
		t.Error("the writing stage does not read as reasoning-heavy")
	}
	if len(store.events) != 3 {
		t.Errorf("events = %d", len(store.events))
	}
}

func TestRecordCallDropsAnUnattributableCall(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	err := svc.RecordCall(context.Background(), llm.ModelRef{ProviderID: "p", ModelID: "m"}, "", llm.Usage{PromptTokens: 5}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %+v, want none without a job in context", store.events)
	}
}

func TestStageMarksTheObserveCallOnly(t *testing.T) {
	work := Work{Kind: "generate", ObserveModel: "openrouter/vision"}
	if got := work.StageFor(llm.ModelRef{ProviderID: "openrouter", ModelID: "vision"}); got != "observe" {
		t.Errorf("observe stage = %q", got)
	}
	if got := work.StageFor(llm.ModelRef{ProviderID: "openrouter", ModelID: "writer"}); got != "generate" {
		t.Errorf("write stage = %q, want the job kind", got)
	}
}

func TestBalanceReportsItsLotsAndUnlimitedForMaster(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{{ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 50, Remaining: 40}}

	balance, err := svc.BalanceFor(context.Background(), "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Unlimited {
		t.Error("a free account reported unlimited")
	}
	// The separately granted bonus remains; free access opens no monthly grant.
	if balance.Credits != 40 {
		t.Errorf("credits = %d", balance.Credits)
	}
	if len(balance.Lots) != 1 {
		t.Errorf("lots = %+v, want only the bonus", balance.Lots)
	}

	master, err := svc.BalanceFor(context.Background(), "root", plan.Master)
	if err != nil {
		t.Fatal(err)
	}
	if !master.Unlimited || len(master.Lots) != 0 {
		t.Errorf("master balance = %+v, want unlimited with no lots", master)
	}
}

// QUOTA-58: a redemption opens one expiring voucher lot and refuses a lot it could not
// describe — no account, no credits, or no expiry.
func TestOpenVoucherLotOpensOneExpiringVoucherLot(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	expires := now.Add(30 * 24 * time.Hour)

	id, err := svc.OpenVoucherLot(ctx, "alice", 1150, expires)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.lots) != 1 {
		t.Fatalf("lots = %+v, want one", store.lots)
	}
	lot := store.lots[0]
	if lot.ID != id || !strings.HasPrefix(id, "voucher:") || lot.Kind != LotVoucher ||
		lot.Granted != 1150 || lot.Remaining != 1150 || lot.ExpiresAt == nil || !lot.ExpiresAt.Equal(expires) {
		t.Fatalf("voucher lot = %+v", lot)
	}

	for _, tc := range []struct {
		name    string
		user    string
		credits int
		expires time.Time
	}{
		{"no account", "", 10, expires},
		{"no credits", "alice", 0, expires},
		{"no expiry", "alice", 10, time.Time{}},
	} {
		if _, err := svc.OpenVoucherLot(ctx, tc.user, tc.credits, tc.expires); err == nil {
			t.Errorf("%s: opened a voucher lot", tc.name)
		}
	}
}

// QUOTA-64: a stage's figure is the upper median of its posts' credits, each converted at its
// own admission rate, over the last 30 days; the sample behind it decides eligibility, and a
// row whose rate cannot convert is left out rather than guessed.
func TestRecentPostFiguresTakeTheUpperMedianOverTheWindow(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	svc, store := newTestService(t, now)
	rate := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29", ReferenceE4: 14_000_000, AppliedE4: 14_000_000}
	write := llm.ModelRef{ProviderID: "openrouter", ModelID: "vendor/writer"}
	users := []string{"a", "b", "c", "a", "b", "c", "a", "b", "c", "a"}
	for i, user := range users {
		// 100,000 micro-USD steps: 140, 280, ... 1,400 credits.
		store.postStageCosts = append(store.postStageCosts, PostStageCost{
			JobID: fmt.Sprintf("job-%d", i), UserID: user, Stage: "write", Model: write,
			CostMicrousd: int64(i+1) * 100_000, Rate: rate,
		})
	}
	store.postStageCosts = append(store.postStageCosts, PostStageCost{
		JobID: "unfrozen", UserID: "d", Stage: "write", Model: write, CostMicrousd: 1, Rate: plan.RateSnapshot{},
	})

	figures, err := svc.RecentPostFigures(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(-plan.PostFigureWindow); !store.postStageSince.Equal(want) {
		t.Fatalf("window since = %s, want %s", store.postStageSince, want)
	}
	got := figures[StageModel{Stage: "write", Model: write}]
	// Ten values 140..1,400: the middle pair 700 and 840 give 770.
	if got.Credits != 770 || got.Posts != 10 || got.Accounts != 3 || !got.Eligible() {
		t.Fatalf("figure = %+v, want 770 from 10 posts by 3 accounts, eligible", got)
	}

	store.postStageCosts = store.postStageCosts[:9]
	thin, err := svc.RecentPostFigures(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if thin[StageModel{Stage: "write", Model: write}].Eligible() {
		t.Fatal("nine posts read as eligible")
	}
}
