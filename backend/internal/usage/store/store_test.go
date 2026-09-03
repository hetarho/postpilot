package store_test

import (
	"context"
	"errors"
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
	handle, err := db.Open(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO users (id, password_hash, plan, created_at) VALUES ('alice','hash','free',?)",
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return usage.NewService(usagestore.New(handle.Writer, handle.Reader), pricedModels{}, maxCompletion)
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
	}, false); err != nil {
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
