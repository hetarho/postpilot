package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

func TestFrozenFXFaultCompensationIsSeparateAndOnceOnly(t *testing.T) {
	_, handle := newServiceWithDB(t)
	ctx := context.Background()
	now := time.Now()
	kst := time.FixedZone("Asia/Seoul", 9*60*60)
	rates := &datedRates{data: map[string]int64{}}
	for i := 1; i <= 7; i++ {
		rates.data[now.In(kst).AddDate(0, 0, -i).Format(time.DateOnly)] = 13_600_000
	}
	store := usagestore.New(handle.Writer, handle.Reader)
	service := usage.NewService(store, pricedModels{}, maxCompletion,
		fixedAnchors{anchor: now.Add(-3 * time.Hour)}).
		WithRateSelector(usage.NewRateSelector(rates, store))
	start := func(job string, tier plan.Plan) {
		t.Helper()
		if err := service.Hold(ctx, usage.Start{UserID: "alice", Kind: "generate", JobID: job,
			Plan: tier, Calls: []usage.PlannedCall{{Ref: pricedRef, Count: 1}}}); err != nil {
			t.Fatalf("hold %s: %v", job, err)
		}
	}
	record := func(job string, microusd int) {
		t.Helper()
		if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO usage_events
            (user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
            VALUES ('alice','generate',?,'write','test/model',1,1,?,'reported',?)`,
			job, microusd, now.UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	start("unknown-fault", plan.Basic)
	called := len(rates.calls)
	rates.err = errors.New("official source unavailable")
	start("unknown-fault", plan.Basic)
	if len(rates.calls) != called {
		t.Fatal("admitted retry fetched a new FX reference")
	}
	rates.err = nil
	record("unknown-fault", 2_500)
	record("unknown-fault", 2_500) // ceil((0.0025+0.0025) USD * 1360)=7 once, not 4+4.
	var reference, applied sql.NullInt64
	var publication sql.NullString
	if err := handle.Reader.QueryRowContext(ctx, `SELECT fx_reference_e4,fx_applied_e4,fx_publication_date
        FROM usage_admissions WHERE job_id='unknown-fault'`).Scan(&reference, &applied, &publication); err != nil {
		t.Fatal(err)
	}
	if reference.Int64 != 13_600_000 || applied.Int64 != 13_600_000 || !publication.Valid {
		t.Fatalf("unfrozen FX: reference=%v applied=%v date=%v", reference, applied, publication)
	}
	if _, err := handle.Writer.ExecContext(ctx, `CREATE TRIGGER reject_fx_settlement BEFORE UPDATE OF settled_at
      ON usage_admissions BEGIN SELECT RAISE(ABORT, 'settlement unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err := service.SettleCause(ctx, "unknown-fault", usage.OutcomeFailed, "unknown"); err == nil {
		t.Fatal("failed settlement should roll back its compensation lot")
	}
	var premature int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_lots WHERE id='compensation:unknown-fault'").Scan(&premature); err != nil || premature != 0 {
		t.Fatalf("failed settlement compensation=%d err=%v", premature, err)
	}
	if _, err := handle.Writer.ExecContext(ctx, "DROP TRIGGER reject_fx_settlement"); err != nil {
		t.Fatal(err)
	}
	if err := service.SettleCause(ctx, "unknown-fault", usage.OutcomeFailed, "unknown"); err != nil {
		t.Fatal(err)
	}
	if err := service.SettleCause(ctx, "unknown-fault", usage.OutcomeFailed, "unknown"); err != nil {
		t.Fatal(err)
	}
	var charged, compensated int
	var cause, expires string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT settled_credits,compensation_credits,settlement_cause,compensation_expires_at
        FROM usage_admissions WHERE job_id='unknown-fault'`).Scan(&charged, &compensated, &cause, &expires); err != nil {
		t.Fatal(err)
	}
	if charged != 7 || compensated != 4 || cause != "unknown" {
		t.Fatalf("fault settlement: charged=%d compensated=%d cause=%s", charged, compensated, cause)
	}
	var count, remaining int
	if err := handle.Reader.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(remaining),0) FROM credit_lots
        WHERE id='compensation:unknown-fault' AND kind='compensation'`).Scan(&count, &remaining); err != nil {
		t.Fatal(err)
	}
	if count != 1 || remaining != 4 {
		t.Fatalf("compensation lot count=%d remaining=%d", count, remaining)
	}
	parsed, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || parsed.Before(now.Add(7*24*time.Hour)) {
		t.Fatalf("compensation expires=%s err=%v", expires, err)
	}
	for _, tc := range []struct {
		job, cause string
		tier       plan.Plan
		cost       int
		want       int
	}{
		{"provider-fault", "provider", plan.Basic, 5_000, 0},
		{"zero-fault", "service", plan.Basic, 0, 0},
		{"master-fault", "service", plan.Master, 5_000, 0},
	} {
		t.Run(tc.job, func(t *testing.T) {
			start(tc.job, tc.tier)
			if tc.cost > 0 {
				record(tc.job, tc.cost)
			} else if tc.job == "zero-fault" {
				if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO usage_events
              (user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
              VALUES ('alice','generate',?,'write','test/model',1,1,5000,'unavailable',?)`,
					tc.job, now.UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
			}
			if err := service.SettleCause(ctx, tc.job, usage.OutcomeFailed, tc.cause); err != nil {
				t.Fatal(err)
			}
			var compensation sql.NullInt64
			if err := handle.Reader.QueryRowContext(ctx, `SELECT compensation_credits FROM usage_admissions WHERE job_id=?`, tc.job).Scan(&compensation); err != nil {
				t.Fatal(err)
			}
			if compensation.Int64 != int64(tc.want) || compensation.Valid != (tc.want > 0) {
				t.Fatal(fmt.Sprintf("%s compensation=%v want=%d", tc.job, compensation, tc.want))
			}
			if tc.tier == plan.Master {
				var shadow, debits int
				if err := handle.Reader.QueryRowContext(ctx, "SELECT settled_credits FROM usage_admissions WHERE job_id=?", tc.job).Scan(&shadow); err != nil {
					t.Fatal(err)
				}
				if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_hold_lots WHERE job_id=?", tc.job).Scan(&debits); err != nil {
					t.Fatal(err)
				}
				if shadow != 7 || debits != 0 {
					t.Fatalf("master reference=%d debited lots=%d", shadow, debits)
				}
			}
		})
	}
}

func TestMissingOfficialFXBlocksNewPaidWorkButNotFreeOnlyWork(t *testing.T) {
	_, handle := newServiceWithDB(t)
	ctx := context.Background()
	source := &datedRates{err: errors.New("official source unavailable")}
	store := usagestore.New(handle.Writer, handle.Reader)
	service := usage.NewService(store, pricedModels{}, maxCompletion,
		fixedAnchors{anchor: time.Now().Add(-time.Hour)}).
		WithRateSelector(usage.NewRateSelector(source, store))
	free := usage.Start{UserID: "alice", Kind: "generate", JobID: "free-only",
		Plan: plan.Free, Calls: []usage.PlannedCall{{Ref: llm.ModelRef{ProviderID: "free", ModelID: "free"}, Count: 1}}}
	if err := service.Hold(ctx, free); err != nil {
		t.Fatal(err)
	}
	if len(source.calls) != 0 {
		t.Fatal("all-free work fetched an exchange rate")
	}
	var held int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT hold_credits FROM usage_admissions WHERE job_id='free-only'").Scan(&held); err != nil || held != 0 {
		t.Fatalf("free hold=%d err=%v", held, err)
	}
	paid := usage.Start{UserID: "alice", Kind: "generate", JobID: "paid-without-rate",
		Plan: plan.Basic, Calls: []usage.PlannedCall{{Ref: pricedRef, Count: 1}}}
	if err := service.Hold(ctx, paid); !errors.Is(err, usage.ErrRateUnavailable) {
		t.Fatalf("missing FX paid hold error=%v", err)
	}
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM usage_admissions WHERE job_id='paid-without-rate'").Scan(&held); err != nil || held != 0 {
		t.Fatalf("refused paid hold persisted=%d err=%v", held, err)
	}
}

func TestFrozenFXCancellationDebitsConfirmedCostOnly(t *testing.T) {
	_, handle := newServiceWithDB(t)
	ctx := context.Background()
	now := time.Now()
	rates := &datedRates{data: map[string]int64{}}
	for i := 1; i <= 7; i++ {
		key := now.In(time.FixedZone("Asia/Seoul", 9*60*60)).AddDate(0, 0, -i).Format(time.DateOnly)
		rates.data[key] = 13_600_000
	}
	store := usagestore.New(handle.Writer, handle.Reader)
	service := usage.NewService(store, pricedModels{}, maxCompletion,
		fixedAnchors{anchor: now.Add(-2 * time.Hour)}, "generate_clip").
		WithRateSelector(usage.NewRateSelector(rates, store))
	rate, err := service.SelectRate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	policy := llm.CallPolicy{Ref: pricedRef, Stage: "write", CompletionTokens: 32768,
		InputUSDPerMillion: "0.15", OutputUSDPerMillion: "0"}
	approval := &usage.Reservation{CancellationPolicyVersion: 1, ApprovedMaxCredits: 100, Rate: rate,
		Calls: []usage.PricedCall{{Policy: policy, Count: 1}, {Policy: policy, Count: 1}}}
	for _, job := range []string{"cancel-with-cost", "cancel-without-cost"} {
		if err := service.Hold(ctx, usage.Start{UserID: "alice", Kind: "generate_clip", JobID: job,
			Plan: plan.Basic, Approval: approval, Calls: []usage.PlannedCall{{Ref: pricedRef, Count: 2}}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO usage_events
      (user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
      VALUES ('alice','generate_clip','cancel-with-cost','write','test/model',1,1,5000,'reported',?)`,
		now.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	for job, want := range map[string]int{"cancel-with-cost": 7, "cancel-without-cost": 0} {
		if err := service.SettleCause(ctx, job, usage.OutcomeCancelled, "owner_cancelled"); err != nil {
			t.Fatal(err)
		}
		var charged, fee int
		if err := handle.Reader.QueryRowContext(ctx, `SELECT settled_credits,cancellation_fee_credits
          FROM usage_admissions WHERE job_id=?`, job).Scan(&charged, &fee); err != nil {
			t.Fatal(err)
		}
		if charged != want || fee != 0 {
			t.Fatalf("%s charged=%d fee=%d want=%d", job, charged, fee, want)
		}
	}
	if err := service.Hold(ctx, usage.Start{UserID: "alice", Kind: "generate_clip", JobID: "completion-race",
		Plan: plan.Basic, Approval: approval, Calls: []usage.PlannedCall{{Ref: pricedRef, Count: 2}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO usage_events
      (user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
      VALUES ('alice','generate_clip','completion-race','write','test/model',1,1,5000,'reported',?)`,
		now.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for _, outcome := range []usage.TerminalOutcome{usage.OutcomeSucceeded, usage.OutcomeCancelled} {
		group.Add(1)
		go func() {
			defer group.Done()
			errs <- service.SettleCause(ctx, "completion-race", outcome, string(outcome))
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var charged, count int
	if err := handle.Reader.QueryRowContext(ctx, "SELECT settled_credits FROM usage_admissions WHERE job_id='completion-race'").Scan(&charged); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_lots WHERE id='compensation:completion-race'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if charged != 7 || count != 0 {
		t.Fatalf("completion race charged=%d compensation lots=%d", charged, count)
	}
}
