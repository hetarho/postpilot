package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

func TestSQLiteClipFailureSettlementEvidenceAndConcurrency(t *testing.T) {
	for _, tc := range []struct {
		name       string
		calls      []llm.Usage
		want       int
		wantSource string
	}{
		{name: "absent"},
		{name: "unknown", calls: []llm.Usage{{}}, wantSource: "unavailable"},
		{name: "reported zero", calls: []llm.Usage{{PromptTokens: 30000, CostReported: true}}, wantSource: "reported"},
		{name: "reported positive", calls: []llm.Usage{{CostMicrousd: 100, CostReported: true}}, want: 3, wantSource: "reported"},
		{name: "estimated from usage", calls: []llm.Usage{{PromptTokens: 100}}, want: 3, wantSource: "estimated"},
		{name: "partial and unknown", calls: []llm.Usage{{CostMicrousd: 100, CostReported: true}, {}}, want: 3, wantSource: "reported"},
		{name: "over reservation", calls: []llm.Usage{{CostMicrousd: 5_000_000, CostReported: true}}, want: 5, wantSource: "reported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, handle := newServiceWithDB(t)
			ctx := context.Background()
			before, err := svc.BalanceFor(ctx, "alice", plan.Free)
			if err != nil {
				t.Fatal(err)
			}
			request := holdFor("clip")
			request.Kind = "generate_clip"
			request.Clip = approvedStoreClip()
			if err := svc.Hold(ctx, request); err != nil {
				t.Fatal(err)
			}
			work := usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: "generate_clip", JobID: "clip"})
			for _, call := range tc.calls {
				// A response without reported usage can still be successful; its cost
				// remains unavailable when a later stage fails. A failed call with usage
				// must retain the same evidence.
				var callErr error
				if call.CostReported || call.PromptTokens > 0 {
					callErr = errors.New("provider failed after usage")
				}
				if err := svc.RecordCall(work, pricedRef, "observe", call, callErr); err != nil {
					t.Fatal(err)
				}
			}
			var workers sync.WaitGroup
			errorsCh := make(chan error, 12)
			for range 12 {
				workers.Go(func() { errorsCh <- svc.Settle(ctx, "clip", usage.OutcomeFailed) })
			}
			workers.Wait()
			close(errorsCh)
			for err := range errorsCh {
				if err != nil {
					t.Fatal(err)
				}
			}
			after, err := svc.BalanceFor(ctx, "alice", plan.Free)
			if err != nil || after.Credits != before.Credits-tc.want {
				t.Fatal(before, after, err)
			}
			var settled, count int
			if err := handle.Reader.QueryRow("SELECT settled_credits FROM usage_admissions WHERE job_id='clip' AND settled_at IS NOT NULL").Scan(&settled); err != nil || settled != tc.want {
				t.Fatal(settled, err)
			}
			if err := handle.Reader.QueryRow("SELECT count(*) FROM usage_events WHERE job_id='clip'").Scan(&count); err != nil || count != len(tc.calls) {
				t.Fatal(count, err)
			}
			if tc.wantSource != "" {
				var source string
				if err := handle.Reader.QueryRow("SELECT cost_source FROM usage_events WHERE job_id='clip' ORDER BY id LIMIT 1").Scan(&source); err != nil || source != tc.wantSource {
					t.Fatal(source, err)
				}
			}
			if err := svc.RecordCall(work, pricedRef, "observe", llm.Usage{CostReported: true, CostMicrousd: 9_000_000}, nil); err != nil {
				t.Fatal(err)
			}
			if err := svc.Settle(ctx, "clip", usage.OutcomeFailed); err != nil {
				t.Fatal(err)
			}
			late, err := svc.BalanceFor(ctx, "alice", plan.Free)
			if err != nil || late.Credits != after.Credits {
				t.Fatal("late evidence charged settled attempt", late, err)
			}
		})
	}
}

func TestSQLiteClipRefundRollbackRestoresOriginalLotsAndExpiry(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	if _, err := svc.BalanceFor(ctx, "alice", plan.Free); err != nil {
		t.Fatal(err)
	}
	// The five-credit hold spans all three lot kinds.
	if _, err := handle.Writer.Exec("UPDATE credit_lots SET remaining=2 WHERE user_id='alice' AND kind='monthly'"); err != nil {
		t.Fatal(err)
	}
	insertLot(t, handle, "bonus", "alice", "bonus", 1, nil, time.Now())
	insertLot(t, handle, "purchased", "alice", "purchased", 20, nil, time.Now())
	request := holdFor("clip")
	request.Kind = "generate_clip"
	request.Clip = approvedStoreClip()
	if err := svc.Hold(ctx, request); err != nil {
		t.Fatal(err)
	}
	// A refund restores the original lot even when it expired during processing.
	expired := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := handle.Writer.Exec("UPDATE credit_lots SET expires_at=? WHERE user_id='alice' AND kind='monthly'", expired); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`CREATE TRIGGER reject_clip_settlement BEFORE UPDATE OF settled_at ON usage_admissions BEGIN SELECT RAISE(ABORT, 'test settlement failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, "clip", usage.OutcomeFailed); err == nil {
		t.Fatal("injected settlement failure succeeded")
	}
	var total int
	if err := handle.Reader.QueryRow("SELECT SUM(remaining) FROM credit_lots WHERE user_id='alice'").Scan(&total); err != nil || total != 18 {
		t.Fatal("partial refund escaped transaction", total, err)
	}
	open, err := svc.OpenHolds(ctx)
	if err != nil || len(open) != 1 {
		t.Fatal(open, err)
	}
	if _, err := handle.Writer.Exec("DROP TRIGGER reject_clip_settlement"); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := svc.Settle(canceled, "clip", usage.OutcomeFailed); err == nil {
		t.Fatal("canceled settlement succeeded")
	}
	for range 2 {
		if err := svc.Settle(ctx, "clip", usage.OutcomeFailed); err != nil {
			t.Fatal(err)
		}
	}
	for kind, want := range map[string]int{"monthly": 2, "bonus": 1, "purchased": 20} {
		var remaining int
		if err := handle.Reader.QueryRow("SELECT remaining FROM credit_lots WHERE user_id='alice' AND kind=?", kind).Scan(&remaining); err != nil || remaining != want {
			t.Fatal(kind, remaining, err)
		}
	}
	var expiry string
	if err := handle.Reader.QueryRow("SELECT expires_at FROM credit_lots WHERE user_id='alice' AND kind='monthly'").Scan(&expiry); err != nil || expiry != expired {
		t.Fatal("refund extended expiry", expiry, err)
	}
	open, err = svc.OpenHolds(ctx)
	if err != nil || len(open) != 0 {
		t.Fatal(open, err)
	}
}

func TestSQLiteSettledClipIsNotRetroactivelyWaived(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	request := holdFor("legacy-clip")
	request.Kind = "generate_clip"
	request.Clip = approvedStoreClip()
	if err := svc.Hold(ctx, request); err != nil {
		t.Fatal(err)
	}
	// This is the same durable state as a historical base-only settlement.
	if err := svc.Settle(ctx, request.JobID, usage.OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	before, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, request.JobID, usage.OutcomeFailed); err != nil {
		t.Fatal(err)
	}
	after, err := svc.BalanceFor(ctx, "alice", plan.Free)
	if err != nil || before.Credits != after.Credits {
		t.Fatal(before, after, err)
	}
	var settled int
	if err := handle.Reader.QueryRow("SELECT settled_credits FROM usage_admissions WHERE job_id=?", request.JobID).Scan(&settled); err != nil || settled != 2 {
		t.Fatal(settled, err)
	}
}

func TestSQLiteMasterClipRecordsCostWithoutDebitingLots(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	insertLot(t, handle, "master-purchased", "alice", "purchased", 100, nil, time.Now())
	request := holdFor("master-clip")
	request.Kind, request.Plan = "generate_clip", plan.Master
	request.Clip = approvedStoreClip()
	if err := svc.Hold(ctx, request); err != nil {
		t.Fatal(err)
	}
	work := usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: "generate_clip", JobID: request.JobID})
	if err := svc.RecordCall(work, pricedRef, "observe", llm.Usage{CostMicrousd: 100, CostReported: true}, errors.New("partial failure")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := svc.Settle(ctx, request.JobID, usage.OutcomeFailed); err != nil {
			t.Fatal(err)
		}
	}
	_, remaining := lotRemaining(t, handle, "master-purchased")
	if remaining != 100 {
		t.Fatal("master lot was debited", remaining)
	}
	var settled, debitRows int
	if err := handle.Reader.QueryRow("SELECT settled_credits FROM usage_admissions WHERE job_id=?", request.JobID).Scan(&settled); err != nil || settled != 3 {
		t.Fatal(settled, err)
	}
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM credit_hold_lots WHERE job_id=?", request.JobID).Scan(&debitRows); err != nil || debitRows != 0 {
		t.Fatal(debitRows, err)
	}
}

// Two bounded calls at known frozen rates cost 5 credits, preserving the lot
// split used by the existing failure-settlement regression fixtures.
func approvedStoreClip() *usage.ClipReservation {
	return &usage.ClipReservation{ApprovedMaxCredits: 5, Calls: []usage.PricedCall{
		{Policy: llm.CallPolicy{Ref: pricedRef, Stage: "observe", CompletionTokens: 8192, InputUSDPerMillion: "0.15", OutputUSDPerMillion: "0"}, Count: 1},
		{Policy: llm.CallPolicy{Ref: pricedRef, Stage: "write", CompletionTokens: 32768, InputUSDPerMillion: "0.15", OutputUSDPerMillion: "0"}, Count: 1},
	}}
}
