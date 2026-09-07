package store_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// pricedModels gives one ref a real price, so a hold has something to compute. The rates
// make the hold's assumed shape cost exactly one credit before the multiplier.
type pricedModels struct{}

var pricedRef = llm.ModelRef{ProviderID: "openrouter", ModelID: "priced"}

func (pricedModels) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	if ref != pricedRef {
		return llm.ModelInfo{}, false
	}
	return llm.ModelInfo{Ref: ref, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.7"}, true
}

const maxCompletion = 10_000

// oneCallHold is what one priced call holds: the per-request base plus three credits.
const oneCallHold = 5

func newService(t *testing.T) *usage.Service {
	t.Helper()
	svc, _ := newServiceWithDB(t)
	return svc
}

// newServiceWithDB also hands back the handle, for the cases that need raw SQL: a lot kind
// only a purchase would open, and a CHECK constraint no Go path can reach.
func newServiceWithDB(t *testing.T) (*usage.Service, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, id := range []string{"alice", "bob"} {
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO users (id, password_hash, plan, created_at) VALUES (?,'hash','free',?)",
			id, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
	return usage.NewService(usagestore.New(handle.Writer, handle.Reader), pricedModels{}, maxCompletion), handle
}

// insertLot writes a lot the Go paths cannot open — a purchased one, or a monthly one with a
// chosen expiry — so an ordering rule can be tested against the shapes it exists for.
func insertLot(t *testing.T, handle *db.DB, id, userID, kind string, credits int, expires *time.Time, created time.Time) {
	t.Helper()
	var expiresAt any
	if expires != nil {
		expiresAt = expires.UTC().Format(time.RFC3339Nano)
	}
	_, err := handle.Writer.ExecContext(context.Background(),
		`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, userID, kind, credits, credits, expiresAt, created.UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("insert %s lot: %v", kind, err)
	}
}

func lotRemaining(t *testing.T, handle *db.DB, id string) (granted, remaining int) {
	t.Helper()
	if err := handle.Reader.QueryRow(
		"SELECT granted, remaining FROM credit_lots WHERE id = ?", id,
	).Scan(&granted, &remaining); err != nil {
		t.Fatalf("read lot %s: %v", id, err)
	}
	return granted, remaining
}

func holdFor(jobID string) usage.Start {
	return usage.Start{
		UserID: "alice", Plan: plan.Free, Kind: "generate", JobID: jobID,
		Calls: []usage.PlannedCall{{Ref: pricedRef, Count: 1}},
	}
}

// A5 under real concurrency: reading a balance and spending it are one decision. Without
// BEGIN IMMEDIATE every one of these requests reads the same balance and passes, which is
// how a 50-credit account spends 100.
func TestConcurrentHoldsCannotOverspendOneBalance(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	// The free grant opens on this first access.
	balance, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	affordable := balance.Credits / oneCallHold
	attempts := affordable + 10

	var wg sync.WaitGroup
	results := make([]error, attempts)
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = svc.Hold(context.Background(), holdFor(string(rune('a'+i))))
		}()
	}
	close(start)
	wg.Wait()

	var held, refused int
	for _, err := range results {
		var credits *plan.InsufficientCreditsError
		switch {
		case err == nil:
			held++
		case errors.As(err, &credits):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if held != affordable {
		t.Fatalf("held = %d, want exactly the %d the balance covers", held, affordable)
	}
	if refused != attempts-held {
		t.Fatalf("refused = %d, want %d", refused, attempts-held)
	}

	after, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if after.Credits < 0 {
		t.Fatalf("balance = %d, want never below zero", after.Credits)
	}
	if want := balance.Credits - held*oneCallHold; after.Credits != want {
		t.Fatalf("balance = %d, want %d", after.Credits, want)
	}
}

// The hold and its refund round-trip through the same store the gate writes with, so a
// released hold really is spendable again.
func TestReleaseReturnsTheHoldToTheBalance(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	before, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Hold(ctx, holdFor("job")); err != nil {
		t.Fatal(err)
	}
	if err := svc.Release(ctx, "job"); err != nil {
		t.Fatal(err)
	}

	after, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if after.Credits != before.Credits {
		t.Fatalf("balance = %d, want the released hold back at %d", after.Credits, before.Credits)
	}
}

// Settlement against the real ledger: the hold priced a full completion, the call spent
// almost none of it, and the difference comes back.
func TestSettleRefundsAgainstTheRecordedLedger(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	before, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Hold(ctx, holdFor("job")); err != nil {
		t.Fatal(err)
	}

	// One recorded call that cost a tenth of what the hold assumed.
	work := usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: "generate", JobID: "job"})
	if err := svc.RecordCall(work, pricedRef, "", llm.Usage{
		PromptTokens: 3_000, CompletionTokens: 1_000,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, "job"); err != nil {
		t.Fatal(err)
	}

	after, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	// 3 000 in at $0.1/M plus 1 000 out at $0.7/M is 1 000 micro-USD: under one credit, so
	// the base plus one.
	if want := before.Credits - 3; after.Credits != want {
		t.Fatalf("balance = %d, want %d", after.Credits, want)
	}

	// Settling twice must not refund twice.
	if err := svc.Settle(ctx, "job"); err != nil {
		t.Fatal(err)
	}
	again, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if again.Credits != after.Credits {
		t.Fatalf("balance = %d after a second settle, want the unchanged %d", again.Credits, after.Credits)
	}
}

func TestReasoningTruncationRoundTripsThroughTheLedgerAggregate(t *testing.T) {
	svc := newService(t)
	ctx := usage.WithWork(context.Background(), usage.Work{UserID: "alice", Kind: "generate", JobID: "truncated"})
	err := svc.RecordCall(ctx, pricedRef, llm.StageNameWrite, llm.Usage{
		CompletionTokens: 8192, ReasoningTokens: 8116,
	}, &llm.TruncatedError{ReasoningTokens: 8116, CompletionTokens: 8192})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ReasoningSpendByModel(context.Background(), llm.StageNameWrite)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ReasoningTruncations != 1 {
		t.Fatalf("reasoning spend = %+v, want one truncation", rows)
	}
}

// The database's own guard, exercised directly: a lot cannot be driven below zero even by
// a caller whose arithmetic is stale.
func TestALotCannotBeSpentBelowZero(t *testing.T) {
	svc := newService(t)
	ctx := context.Background()

	balance, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	// Drain it, then keep asking.
	for range balance.Credits/oneCallHold + 5 {
		_ = svc.Hold(ctx, holdFor(time.Now().Format(time.RFC3339Nano)))
	}
	after, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if after.Credits < 0 {
		t.Fatalf("balance = %d, want never below zero", after.Credits)
	}
}

// QUOTA-12 in the case expiry order alone got wrong: a purchased lot that is OLDER than the
// bonus beside it. Ordering by expiry with NULLs last would then fall through to creation
// and spend the paid credits first.
func TestConsumptionWalksLotsByKindBeforeExpiry(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	soon := now.Add(24 * time.Hour)
	later := now.Add(7 * 24 * time.Hour)
	insertLot(t, handle, "purchased", "alice", "purchased", 10, nil, now.Add(-72*time.Hour))
	insertLot(t, handle, "bonus", "alice", "bonus", 10, nil, now.Add(-48*time.Hour))
	insertLot(t, handle, "monthly-b", "alice", "monthly", 10, &later, now.Add(-24*time.Hour))
	insertLot(t, handle, "monthly-a", "alice", "monthly", 10, &soon, now.Add(-12*time.Hour))

	// Five holds at oneCallHold each: both monthly lots and half of the bonus.
	for i := range 5 {
		if err := svc.Hold(ctx, holdFor(fmt.Sprintf("job-%d", i))); err != nil {
			t.Fatalf("hold %d: %v", i, err)
		}
	}

	for _, tc := range []struct {
		lot  string
		want int
	}{
		{"monthly-a", 0},
		{"monthly-b", 0},
		{"bonus", 5},
		{"purchased", 10},
	} {
		if _, remaining := lotRemaining(t, handle, tc.lot); remaining != tc.want {
			t.Errorf("%s remaining = %d, want %d", tc.lot, remaining, tc.want)
		}
	}
}

// The schema is the only thing that can refuse a kind the product does not have.
func TestCreditLotKindIsOneOfThree(t *testing.T) {
	_, handle := newServiceWithDB(t)
	now := time.Now().UTC()

	insertLot(t, handle, "m", "alice", "monthly", 1, &now, now)
	insertLot(t, handle, "b", "alice", "bonus", 1, nil, now)
	insertLot(t, handle, "p", "alice", "purchased", 1, nil, now)

	_, err := handle.Writer.ExecContext(context.Background(),
		`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
		 VALUES ('g', 'alice', 'gift', 1, 1, NULL, ?)`, now.Format(time.RFC3339Nano))
	if err == nil {
		t.Fatal("a fourth lot kind was accepted")
	}
}

// QUOTA-35's credit side: the cycle already running grows rather than a second monthly lot
// opening beside it, and it grows on both sides so a partly spent lot still satisfies
// `remaining <= granted`.
func TestTopUpMonthlyLotRaisesTheRunningCycle(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	expires := now.Add(14 * 24 * time.Hour)

	insertLot(t, handle, "monthly", "alice", "monthly", 10, &expires, now.Add(-time.Hour))
	if err := svc.Hold(ctx, holdFor("job-1")); err != nil {
		t.Fatalf("hold: %v", err)
	}

	if err := svc.TopUpMonthlyLot(ctx, "alice", 7); err != nil {
		t.Fatalf("TopUpMonthlyLot: %v", err)
	}
	granted, remaining := lotRemaining(t, handle, "monthly")
	if granted != 17 || remaining != 10-oneCallHold+7 {
		t.Errorf("lot = %d/%d granted/remaining, want 17 and %d", granted, remaining, 10-oneCallHold+7)
	}

	var lots int
	if err := handle.Reader.QueryRow(
		"SELECT COUNT(*) FROM credit_lots WHERE user_id = 'alice' AND kind = 'monthly'",
	).Scan(&lots); err != nil {
		t.Fatal(err)
	}
	if lots != 1 {
		t.Errorf("monthly lots = %d, want the running one raised rather than a second opened", lots)
	}

	if err := svc.TopUpMonthlyLot(ctx, "alice", 0); err == nil {
		t.Error("a non-positive top-up was accepted")
	}
}

// An account with no current monthly lot is a no-op: its next request opens one at the new
// tier's size anyway, and inventing one here would double the grant.
func TestTopUpMonthlyLotIsANoOpWithNoRunningCycle(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	lapsed := time.Now().UTC().Add(-24 * time.Hour)
	insertLot(t, handle, "lapsed", "bob", "monthly", 10, &lapsed, lapsed.Add(-30*24*time.Hour))

	if err := svc.TopUpMonthlyLot(ctx, "bob", 500); err != nil {
		t.Fatalf("TopUpMonthlyLot with no running cycle = %v, want nil", err)
	}
	if granted, _ := lotRemaining(t, handle, "lapsed"); granted != 10 {
		t.Errorf("lapsed lot granted = %d, want it untouched at 10", granted)
	}
}
