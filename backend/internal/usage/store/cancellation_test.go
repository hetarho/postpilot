package store_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

func TestSQLiteCancellationSettlementIsOnceBoundedAndSeparateFromUsage(t *testing.T) {
	for _, tc := range []struct {
		name            string
		reservation     int
		cost            int64
		confirmed, fee  int
		master, unknown bool
	}{
		{name: "confirmed", reservation: 100, cost: 60000, confirmed: 20, fee: 40},
		{name: "no usage", reservation: 5, fee: 3},
		{name: "unknown", reservation: 5, fee: 3, unknown: true},
		{name: "full", reservation: 5, cost: 10000, confirmed: 5},
		{name: "over ceiling", reservation: 5, cost: math.MaxInt64, confirmed: 5},
		{name: "zero reservation"},
		{name: "master", reservation: 100, cost: 60000, confirmed: 20, fee: 40, master: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, handle := newServiceWithDB(t)
			ctx := context.Background()
			st := usagestore.New(handle.Writer, handle.Reader)
			insertLot(t, handle, "original", "alice", "purchased", 200, nil, time.Now())
			err := st.InWriteTx(ctx, func(tx usage.Store) error {
				if err := tx.InsertAdmission(ctx, usage.Admission{UserID: "alice", Kind: "generate_clip", JobID: "clip", HoldCredits: tc.reservation, ApprovedMaxCredits: &tc.reservation, CancellationPolicyVersion: 1, CreatedAt: time.Now()}); err != nil {
					return err
				}
				if tc.master || tc.reservation == 0 {
					return nil
				}
				if err := tx.SpendFromLot(ctx, "original", tc.reservation); err != nil {
					return err
				}
				return tx.InsertHoldDebits(ctx, "clip", []usage.LotDebit{{LotID: "original", Credits: tc.reservation}})
			})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			if tc.cost > 0 || tc.unknown {
				source := llm.CostReported
				if tc.unknown {
					source = llm.CostUnavailable
				}
				if err := st.InsertEvent(ctx, usage.Event{UserID: "alice", Kind: "generate_clip", JobID: "clip", Stage: "observe", Model: pricedRef.String(), CostMicrousd: tc.cost, CostSource: source, CreatedAt: time.Now()}); err != nil {
					t.Fatal(err)
				}
				calls = 1
			}
			var wg sync.WaitGroup
			errs := make(chan error, 12)
			for range 12 {
				wg.Go(func() { errs <- svc.Settle(ctx, "clip", usage.OutcomeCancelled) })
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			a, err := svc.ClipAccounting(ctx, "alice", "clip")
			if err != nil || a == nil {
				t.Fatal(a, err)
			}
			want := tc.confirmed + tc.fee
			if !a.Settled || a.SettlementReason != "cancelled" || a.NominalReservation == nil || *a.NominalReservation != tc.reservation {
				t.Fatal(a)
			}
			if tc.master {
				if *a.FinalCharge != 0 || *a.Refund != 0 || *a.ShadowCharge != want || *a.ShadowConfirmedCharge != tc.confirmed || *a.ShadowCancellationFee != tc.fee {
					t.Fatal(a)
				}
				want = 0
			} else if *a.FinalCharge != want || *a.Refund != tc.reservation-want || *a.ConfirmedCharge != tc.confirmed || *a.CancellationFee != tc.fee {
				t.Fatal(a)
			}
			_, remaining := lotRemaining(t, handle, "original")
			if remaining != 200-want {
				t.Fatal("incorrect lot debit", remaining)
			}
			var events int
			if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_events WHERE job_id='clip'").Scan(&events); err != nil || events != calls {
				t.Fatal("fee became provider usage", events, err)
			}
			if other, err := svc.ClipAccounting(ctx, "bob", "clip"); err != nil || other != nil {
				t.Fatal("foreign accounting", other, err)
			}
			if err := svc.Settle(ctx, "clip", usage.OutcomeFailed); err != nil {
				t.Fatal(err)
			}
			_, again := lotRemaining(t, handle, "original")
			if again != remaining {
				t.Fatal("changed terminal settlement")
			}
		})
	}
}

func TestSQLiteCancellationPreservesOldPoliciesAndMissingReservations(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	request := holdFor("legacy")
	request.Kind, request.Clip = "generate_clip", approvedStoreClip()
	if err := svc.Hold(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, "legacy", usage.OutcomeCancelled); !errors.Is(err, usage.ErrSettlementOutcome) {
		t.Fatal("legacy fee accepted", err)
	}
	if err := svc.Settle(ctx, "before-hold", usage.OutcomeCancelled); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, "manual-render", usage.OutcomeCancelled); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions").Scan(&n); err != nil || n != 1 {
		t.Fatal("invented admission", n, err)
	}
}

func TestSQLiteCancellationRefundRollbackKeepsOriginalExpiredLots(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := t.Context()
	if _, err := svc.BalanceFor(ctx, "alice", plan.Free); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec("UPDATE credit_lots SET remaining=2 WHERE user_id='alice' AND kind='monthly'"); err != nil {
		t.Fatal(err)
	}
	insertLot(t, handle, "bonus", "alice", "bonus", 1, nil, time.Now())
	insertLot(t, handle, "purchased", "alice", "purchased", 20, nil, time.Now())
	request := holdFor("cancelled")
	request.Kind = "generate_clip"
	request.Clip = approvedStoreClip()
	request.Clip.CancellationPolicyVersion = 1
	if err := svc.Hold(ctx, request); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := handle.Writer.Exec("UPDATE credit_lots SET expires_at=? WHERE user_id='alice' AND kind='monthly'", expiry); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`CREATE TRIGGER reject_cancel_settlement BEFORE UPDATE OF settled_at ON usage_admissions BEGIN SELECT RAISE(ABORT,'fault'); END`); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, "cancelled", usage.OutcomeCancelled); err == nil {
		t.Fatal("fault failed to roll back")
	}
	var total int
	if err := handle.Reader.QueryRow("SELECT SUM(remaining) FROM credit_lots WHERE user_id='alice'").Scan(&total); err != nil || total != 18 {
		t.Fatal("refund escaped transaction", total, err)
	}
	if _, err := handle.Writer.Exec("DROP TRIGGER reject_cancel_settlement"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := svc.Settle(ctx, "cancelled", usage.OutcomeCancelled); err != nil {
			t.Fatal(err)
		}
	}
	var remaining int
	var actualExpiry string
	if err := handle.Reader.QueryRow("SELECT remaining,expires_at FROM credit_lots WHERE user_id='alice' AND kind='monthly'").Scan(&remaining, &actualExpiry); err != nil || remaining != 2 || actualExpiry != expiry {
		t.Fatal("refund moved or renewed expired credits", remaining, actualExpiry, err)
	}
	_, purchased := lotRemaining(t, handle, "purchased")
	if purchased != 18 {
		t.Fatal("refund moved into purchased credits", purchased)
	}
	a, err := svc.ClipAccounting(ctx, "alice", "cancelled")
	if err != nil || *a.Refund != 2 || *a.FinalCharge != 3 {
		t.Fatal(a, err)
	}
}
