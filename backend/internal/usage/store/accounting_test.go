package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

func TestClipAccountingDistinguishesPendingZeroLegacyAndMaster(t *testing.T) {
	for _, tier := range []plan.Plan{plan.Free, plan.Master} {
		t.Run(string(tier), func(t *testing.T) {
			svc, _ := newServiceWithDB(t)
			ctx := context.Background()
			if a, err := svc.ClipAccounting(ctx, "alice", "clip"); err != nil || a != nil {
				t.Fatal(a, err)
			}
			start := holdFor("clip")
			start.Kind = "generate_clip"
			start.Plan = tier
			start.Clip = approvedStoreClip()
			if err := svc.Hold(ctx, start); err != nil {
				t.Fatal(err)
			}
			a, err := svc.ClipAccounting(ctx, "alice", "clip")
			if err != nil || a.Approved == nil || *a.Approved != 5 || a.Reserved == nil || a.FinalCharge != nil || a.Refund != nil || a.Settled {
				t.Fatal(a, err)
			}
			want := 5
			if tier == plan.Master {
				want = 0
			}
			if *a.Reserved != want || a.Exempt != (tier == plan.Master) {
				t.Fatal(a)
			}
			if a, err := svc.ClipAccounting(ctx, "bob", "clip"); err != nil || a != nil {
				t.Fatal("foreign accounting", a, err)
			}
			if err := svc.Settle(ctx, "clip", usage.OutcomeFailed); err != nil {
				t.Fatal(err)
			}
			a, err = svc.ClipAccounting(ctx, "alice", "clip")
			if err != nil || !a.Settled || a.FinalCharge == nil || *a.FinalCharge != 0 || a.Refund == nil || *a.Refund != want {
				t.Fatal(a, err)
			}
			if tier == plan.Master && (a.ShadowCharge == nil || *a.ShadowCharge != 0) {
				t.Fatal("master shadow omitted", a)
			}
		})
	}
	svc, handle := newServiceWithDB(t)
	if _, err := handle.Writer.Exec("INSERT INTO usage_admissions(user_id,kind,job_id,hold_credits,created_at,settled_at,settled_credits) VALUES('alice','generate_clip','legacy',2,'2026-09-01T00:00:00Z','2026-09-01T00:01:00Z',2)"); err != nil {
		t.Fatal(err)
	}
	a, err := svc.ClipAccounting(context.Background(), "alice", "legacy")
	if err != nil || a.Approved != nil {
		t.Fatal("invented legacy approval", a, err)
	}
}

func TestSQLiteCeilingRefusalAndOverageStayWithinApproval(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	start := holdFor("clip")
	start.Kind = "generate_clip"
	start.Clip = approvedStoreClip()
	start.Clip.ApprovedMaxCredits = 4
	var ceiling *usage.CreditCeilingError
	if err := svc.Hold(ctx, start); !errors.As(err, &ceiling) || ceiling.Required != 5 {
		t.Fatal(err)
	}
	for _, table := range []string{"credit_lots", "usage_admissions", "credit_hold_lots"} {
		var n int
		if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatal("refused transaction wrote", table, n, err)
		}
	}
	start.Clip.ApprovedMaxCredits = 5
	if err := svc.Hold(ctx, start); err != nil {
		t.Fatal(err)
	}
	if err := svc.Record(ctx, usage.Call{UserID: "alice", Kind: "generate_clip", JobID: "clip", Stage: "observe", Model: pricedRef, Usage: llm.Usage{CostReported: true, CostMicrousd: 9_000_000}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(ctx, "clip", usage.OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	a, err := svc.ClipAccounting(ctx, "alice", "clip")
	if err != nil || *a.FinalCharge != 5 || *a.Refund != 0 || *a.Reserved != 5 || *a.Approved != 5 {
		t.Fatal(a, err)
	}
	var cost int
	if err := handle.Reader.QueryRow("SELECT SUM(cost_microusd) FROM usage_events WHERE job_id='clip'").Scan(&cost); err != nil || cost != 9_000_000 {
		t.Fatal("supplier overage lost", cost, err)
	}
}
