package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

func TestOrdinaryBillingReadsDoNotBorrowTheWriter(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "read-pool.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(t.Context(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	store := billingstore.New(handle.Writer, handle.Reader)
	writer, err := handle.Writer.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	for name, read := range map[string]func(context.Context) error{
		"refund list":        func(ctx context.Context) error { _, err := store.Refunds(ctx, "alice"); return err },
		"refund detail":      func(ctx context.Context) error { _, _, err := store.RefundRequest(ctx, "missing"); return err },
		"refundable payment": func(ctx context.Context) error { _, _, err := store.RefundPayment(ctx, "alice", "missing"); return err },
		"open refund":        func(ctx context.Context) error { _, err := store.OpenRefundForOrder(ctx, "missing"); return err },
		"processing refunds": func(ctx context.Context) error { _, err := store.ProcessingRefundIDs(ctx, time.Now(), 100); return err },
		"reviewed evidence":  func(ctx context.Context) error { _, _, err := store.ReviewedEvidence(ctx, "missing"); return err },
		"confirmed total":    func(ctx context.Context) error { _, err := store.ConfirmedRefundTotal(ctx, "missing"); return err },
		"quote":              func(ctx context.Context) error { _, _, err := store.Quote(ctx, "missing"); return err },
		"intent":             func(ctx context.Context) error { _, _, err := store.Intent(ctx, "missing"); return err },
		"pending intent":     func(ctx context.Context) error { _, _, err := store.PendingIntent(ctx, "alice"); return err },
		"due intents":        func(ctx context.Context) error { _, err := store.DueIntents(ctx, time.Now()); return err },
		"review intents":     func(ctx context.Context) error { _, err := store.ReviewIntents(ctx, 100); return err },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := read(ctx); err != nil {
				t.Fatalf("read queued behind the borrowed sole writer: %v", err)
			}
		})
	}
}

func TestBillingTransactionReadsItsOwnRawWritesAndRollsBack(t *testing.T) {
	at := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	h, _, _ := fixedService(t, at)
	quote := billing.QuoteRecord{ID: "transaction-quote", UserID: "alice", Tier: plan.Basic,
		Term: billing.TermMonthly, KRW: 4900, AppliedNow: true, EffectiveAt: at,
		SubscriptionUpdatedAt: at, QuotedAt: at, ExpiresAt: at.Add(time.Minute)}
	rollback := errors.New("rollback fixture")
	assertScoped := func(t *testing.T, scoped billing.Store) error {
		t.Helper()
		if err := scoped.PutQuote(t.Context(), quote); err != nil {
			return err
		}
		got, found, err := scoped.Quote(t.Context(), quote.ID)
		if err != nil || !found || got.KRW != quote.KRW {
			t.Fatalf("transaction did not read its own quote: %+v found=%v err=%v", got, found, err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if _, found, err := h.store.Quote(ctx, quote.ID); err != nil || found {
			t.Fatalf("ordinary read did not see the committed snapshot: found=%v err=%v", found, err)
		}
		return rollback
	}
	t.Run("NewTx", func(t *testing.T) {
		tx, err := h.handle.Writer.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		if err := assertScoped(t, billingstore.NewTx(tx)); !errors.Is(err, rollback) {
			t.Fatal(err)
		}
	})
	t.Run("InWriteTx", func(t *testing.T) {
		err := h.store.InWriteTx(t.Context(), func(scoped billing.Store, _ billing.Credits, _ billing.Plans) error {
			return assertScoped(t, scoped)
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
	})
	if _, found, err := h.store.Quote(t.Context(), quote.ID); err != nil || found {
		t.Fatalf("rolled-back quote survived: found=%v err=%v", found, err)
	}
}
