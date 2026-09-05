package usage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// fakeStore is an in-memory usage.Store. These tests are about the hold rules, the
// consumption order and the settlement arithmetic, not about SQL — but the two guards the
// real statements carry (a lot never drops below zero, never rises above its grant) are
// reproduced here, because those are the invariant rather than an implementation detail.
type fakeStore struct {
	lots        []Lot
	admissions  []Admission
	holdDebits  map[string][]LotDebit
	settled     map[string]int
	events      []Event
	lotSeq      int
	spendCalls  int
	failOnSpend error
}

func newFakeStore() *fakeStore {
	return &fakeStore{holdDebits: map[string][]LotDebit{}, settled: map[string]int{}}
}

// InWriteTx is a pass-through here: these tests assert the rules, and the real store's
// BEGIN IMMEDIATE is what makes them hold under concurrency.
func (f *fakeStore) InWriteTx(_ context.Context, fn func(Store) error) error { return fn(f) }

func (f *fakeStore) LotsInConsumptionOrder(_ context.Context, userID string, now time.Time) ([]Lot, error) {
	var out []Lot
	for _, lot := range f.lots {
		if lot.UserID != userID || lot.Remaining <= 0 || lot.Expired(now) {
			continue
		}
		out = append(out, lot)
	}
	// expiry ascending, NULLs last, then creation — the ORDER BY the query spells out.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && lotBefore(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

func lotBefore(a, b Lot) bool {
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

func (f *fakeStore) InsertLotIfAbsent(ctx context.Context, lot Lot) error {
	for _, existing := range f.lots {
		if existing.ID == lot.ID {
			return nil
		}
	}
	return f.InsertLot(ctx, lot)
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

func (f *fakeStore) MarkSettled(_ context.Context, jobID string, credits int, _ time.Time) error {
	f.settled[jobID] = credits
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

func (f *fakeStore) SumCostForJob(_ context.Context, jobID string) (int64, error) {
	var total int64
	for _, event := range f.events {
		if event.JobID == jobID {
			total += event.CostMicrousd
		}
	}
	return total, nil
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

// seoulNoon is a fixed instant inside one Asia/Seoul month, far from either boundary.
var seoulNoon = time.Date(2026, 9, 15, 3, 0, 0, 0, time.UTC) // 15 Sep, 12:00 KST

// cheap costs 2300 micro-USD at the hold's assumed shape, which charges 3 credits.
var cheapRef = llm.ModelRef{ProviderID: "openrouter", ModelID: "cheap"}

// The hold prices 30 000 prompt tokens plus the completion cap; these rates make that
// exactly 10 000 micro-USD, i.e. one credit of cost and so 5 credits charged at 3x.
var pricedModels = fakeModels{
	cheapRef: {Ref: cheapRef, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"},
}

const maxCompletion = 10_000

func newTestService(t *testing.T, now time.Time) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	svc := NewService(store, pricedModels, maxCompletion)
	svc.now = func() time.Time { return now }
	seq := 0
	svc.newID = func() string { seq++; return fmt.Sprintf("lot-new-%d", seq) }
	return svc, store
}

// openMonthly is a monthly lot already open for the current month, so a test about the
// hold itself does not also trigger the renewal that a first access performs.
func openMonthly(userID string, remaining int) Lot {
	end := plan.NextRenewal(seoulNoon)
	return Lot{
		ID: "monthly-open", UserID: userID, Kind: LotMonthly,
		Granted: remaining, Remaining: remaining, ExpiresAt: &end,
	}
}

func holdStart(userID string, tier plan.Plan, jobID string, calls ...PlannedCall) Start {
	return Start{UserID: userID, Plan: tier, Kind: "generate", JobID: jobID, Calls: calls}
}

// The hold's assumed shape prices one cheap call at 10 000 micro-USD, so Charge gives
// 2 + 3 = 5 credits.
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
	// Four calls of cost, one per-request base: 2 + 4*3.
	if got, want := store.admissions[0].HoldCredits, 14; got != want {
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
	if refusal.RenewsAt.IsZero() {
		t.Error("the refusal did not name when it lifts")
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
	monthEnd := plan.NextRenewal(seoulNoon)
	later := monthEnd.AddDate(0, 2, 0)
	store.lots = []Lot{
		{ID: "no-expiry", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100},
		{ID: "later-bonus", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100, ExpiresAt: &later},
		{ID: "monthly", UserID: "alice", Kind: LotMonthly, Granted: 4, Remaining: 4, ExpiresAt: &monthEnd},
	}

	if err := svc.Hold(context.Background(), holdStart("alice", plan.Basic, "job-1", PlannedCall{Ref: cheapRef, Count: 1})); err != nil {
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

func TestRenewalOpensTheMonthlyGrantOnAccess(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)

	balance, err := svc.BalanceFor(context.Background(), "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Credits != plan.MonthlyCredits(plan.Basic) {
		t.Errorf("credits = %d, want the basic grant %d", balance.Credits, plan.MonthlyCredits(plan.Basic))
	}
	if !balance.RenewsAt.Equal(plan.NextRenewal(seoulNoon)) {
		t.Errorf("renews at %s, want the month boundary %s", balance.RenewsAt, plan.NextRenewal(seoulNoon))
	}
	if len(store.lots) != 1 {
		t.Fatalf("lots = %+v, want exactly one opened", store.lots)
	}

	// A second read must not mint a second grant.
	if _, err := svc.BalanceFor(context.Background(), "alice", plan.Basic); err != nil {
		t.Fatal(err)
	}
	if len(store.lots) != 1 {
		t.Errorf("lots = %+v, want the same one", store.lots)
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
	if balance.Credits != plan.MonthlyCredits(plan.Basic) {
		t.Errorf("credits = %d, want a fresh grant with nothing carried over", balance.Credits)
	}
	if len(store.lots) != 2 {
		t.Errorf("lots = %d, want the lapsed one kept and a new one opened", len(store.lots))
	}
}

func TestMasterIsNeverRefusedButIsStillHeldAndRecorded(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)

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

	if err := svc.Settle(ctx, "job-1"); err != nil {
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

	// The work actually cost one credit of provider spend: 2 + 3 = 5 charged.
	store.events = append(store.events, Event{UserID: "alice", JobID: "job-1", CostMicrousd: 10_000})

	if err := svc.Settle(ctx, "job-1"); err != nil {
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
	store.events = append(store.events, Event{UserID: "alice", JobID: "job-1", CostMicrousd: 10_000})

	if err := svc.Settle(ctx, "job-1"); err != nil {
		t.Fatal(err)
	}
	after := store.balance("alice", seoulNoon)
	if err := svc.Settle(ctx, "job-1"); err != nil {
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
	store.events = append(store.events, Event{UserID: "alice", JobID: "job-1", CostMicrousd: 5_000_000})

	if err := svc.Settle(ctx, "job-1"); err != nil {
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

func TestSignupBonusIsGrantedOnce(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	ctx := context.Background()

	for range 3 {
		if err := svc.GrantSignupBonus(ctx, "alice", plan.SignupBonusCredits); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.lots) != 1 {
		t.Fatalf("lots = %d, want one bonus however many times provisioning is rerun", len(store.lots))
	}
	if store.lots[0].Granted != plan.SignupBonusCredits || store.lots[0].ExpiresAt != nil {
		t.Errorf("bonus lot = %+v, want %d credits with no expiry", store.lots[0], plan.SignupBonusCredits)
	}
}

func TestRecordCallPricesAndAttributesFromContext(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "z-ai/glm-5.3-flash"}
	store := newFakeStore()
	svc := NewService(store, fakeModels{ref: {
		Ref: ref, InputUSDPerMillion: "0.075", OutputUSDPerMillion: "0.25",
	}}, maxCompletion)
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

	// A failure the provider still billed is the case job 23 preserves usage for.
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
	// The bonus plus the monthly grant renewal opened on this very read.
	if balance.Credits != 40+plan.MonthlyCredits(plan.Free) {
		t.Errorf("credits = %d", balance.Credits)
	}
	if len(balance.Lots) != 2 {
		t.Errorf("lots = %+v, want the bonus and the fresh monthly grant", balance.Lots)
	}

	master, err := svc.BalanceFor(context.Background(), "root", plan.Master)
	if err != nil {
		t.Fatal(err)
	}
	if !master.Unlimited || len(master.Lots) != 0 {
		t.Errorf("master balance = %+v, want unlimited with no lots", master)
	}
}
