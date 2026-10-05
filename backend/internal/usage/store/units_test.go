package store_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type unitChecker struct{ unavailable atomic.Bool }

func (c *unitChecker) ValidateUnitBudget(context.Context, string, plan.Plan, usage.UnitBudget) error {
	if c.unavailable.Load() {
		return usage.ErrUnitPricing
	}
	return nil
}

type noUnitCoverage struct{}

func (noUnitCoverage) AnchorFor(context.Context, string) (time.Time, error) { return time.Now(), nil }
func (noUnitCoverage) CoverageFor(context.Context, string, time.Time) (usage.Coverage, bool, error) {
	return usage.Coverage{}, false, nil
}

func unitFixture(t *testing.T) (*usage.Service, *db.DB, *unitChecker) {
	t.Helper()
	_, handle := newServiceWithDB(t)
	if _, err := handle.Writer.Exec("DELETE FROM credit_lots"); err != nil {
		t.Fatal(err)
	}
	insertLot(t, handle, "original", "alice", "purchased", 200, nil, time.Now())
	store := usagestore.New(handle.Writer, handle.Reader)
	now := time.Now().In(time.FixedZone("Asia/Seoul", 9*60*60))
	rates := &datedRates{data: map[string]int64{}}
	for i := 1; i <= 7; i++ {
		rates.data[now.AddDate(0, 0, -i).Format(time.DateOnly)] = 14_800_000
	}
	checker := &unitChecker{}
	return usage.NewService(store, pricedModels{}, maxCompletion, noUnitCoverage{}, usage.NewRateSelector(rates, store)).WithUnitAccounting(checker), handle, checker
}

func unitRequest() llm.VoiceDesignRequest {
	return llm.VoiceDesignRequest{Model: llm.ModelRef{ProviderID: "speech", ModelID: "design"}, Description: strings.Repeat("description ", 3), PreviewText: strings.Repeat("가", 100)}
}
func unitBudget(t *testing.T, scope string) usage.UnitBudget {
	t.Helper()
	input, err := unitRequest().Input()
	if err != nil {
		t.Fatal(err)
	}
	return usage.UnitBudget{PolicyID: "profile", Revision: 1, ScopeDigest: usage.UnitDigest(scope), Ref: input.Ref, Operation: input.Operation, InputDigest: input.Digest, Count: 1, InputCharacters: input.InputCharacters, AuxiliaryCharacters: input.AuxiliaryCharacters, ParametersDigest: input.ParametersDigest,
		Tariffs: []usage.UnitTariff{{Unit: llm.SpeechUnitCharacterCost, USDPerUnit: "0.0001", Multiplier: "1", MaximumUnits: "1000", UnitsPerInputCharacter: "1"}}, Source: "https://example.com/prices", BoundsSource: "https://example.com/bounds", CheckedAt: time.Now().Add(-time.Minute), Complete: true}
}
func unitStart(t *testing.T, svc *usage.Service, job string, tier plan.Plan, budgets ...usage.UnitBudget) (usage.Start, usage.UnitQuote) {
	t.Helper()
	ctx := t.Context()
	q, err := svc.QuoteUnits(ctx, "alice", tier, "audio", budgets)
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc.ReservationForUnits(ctx, "alice", tier, "audio", usage.UnitApproval{QuoteID: q.ID, ApprovedMaxCredits: &q.MaxCredits, CancellationPolicyVersion: 1}, budgets)
	if err != nil {
		t.Fatal(err)
	}
	s := usage.Start{UserID: "alice", Plan: tier, Kind: "audio", JobID: job, Approval: r}
	for i := range budgets {
		b := budgets[i]
		s.Calls = append(s.Calls, usage.PlannedCall{Ref: b.Ref, Stage: b.Operation, Count: b.Count, Units: &b})
	}
	return s, q
}
func unitWork(ctx context.Context, job string, b usage.UnitBudget) context.Context {
	return usage.WithWork(ctx, usage.Work{UserID: "alice", Kind: "audio", JobID: job, UnitScopeDigest: b.ScopeDigest})
}
func reportUnit(t *testing.T, svc *usage.Service, job string, b usage.UnitBudget, e llm.SpeechEvidence) usage.UnitClaim {
	t.Helper()
	input, _ := unitRequest().Input()
	c, err := svc.AdmitUnitCall(unitWork(t.Context(), job, b), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordUnitCall(t.Context(), c, e); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSQLiteUnitQuotesBindOwnerPricesExactInputAndExpiry(t *testing.T) {
	svc, handle, checker := unitFixture(t)
	b := unitBudget(t, "scope")
	s, q := unitStart(t, svc, "one", plan.Basic, b)
	for _, mutate := range []func(*usage.Start){
		func(s *usage.Start) { s.UserID = "bob" },
		func(s *usage.Start) { s.Approval.UnitQuoteID = "forged" },
		func(s *usage.Start) { s.Approval.ApprovedMaxCredits-- },
		func(s *usage.Start) { s.Calls[0].Units.InputDigest = usage.UnitDigest("changed input") },
		func(s *usage.Start) { s.Calls[0].Units.Tariffs[0].USDPerUnit = "0" },
		func(s *usage.Start) { s.Approval.CancellationPolicyVersion = 0 },
	} {
		copy := s
		approval := *s.Approval
		copy.Approval = &approval
		copy.Calls = append([]usage.PlannedCall(nil), s.Calls...)
		ub := *s.Calls[0].Units
		ub.Tariffs = append([]usage.UnitTariff(nil), ub.Tariffs...)
		copy.Calls[0].Units = &ub
		mutate(&copy)
		if err := svc.Hold(t.Context(), copy); err == nil {
			t.Fatal("forged approval accepted", copy)
		}
	}
	checker.unavailable.Store(true)
	if err := svc.Hold(t.Context(), s); err == nil {
		t.Fatal("changed live price accepted")
	}
	checker.unavailable.Store(false)
	if _, err := handle.Writer.Exec("UPDATE usage_unit_quotes SET quote_json=json_set(quote_json,'$.ExpiresAt',?) WHERE id=?", time.Now().Add(-time.Minute).UTC().Format("2006-01-02T15:04:05.000000000Z07:00"), q.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Hold(t.Context(), s); err == nil {
		t.Fatal("expired quote accepted")
	}
	var count int
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	_, remaining := lotRemaining(t, handle, "original")
	if remaining != 200 {
		t.Fatal("refusal spent credits", remaining)
	}
	bad := b
	bad.Tariffs = append([]usage.UnitTariff(nil), b.Tariffs...)
	bad.Tariffs[0].Unit = llm.SpeechUnitSeconds
	if _, err := svc.QuoteUnits(t.Context(), "alice", plan.Master, "audio", []usage.UnitBudget{bad}); err == nil {
		t.Fatal("master bypassed output bound")
	}
}

func TestSQLiteUnitSettlementPreservesEvidenceCapsUnknownFailureAndMaster(t *testing.T) {
	for _, tc := range []struct {
		name, quantity, reported, cause string
		outcome                         usage.TerminalOutcome
		master                          bool
		want, compensation              int
	}{
		{name: "confirmed", quantity: "5", outcome: usage.OutcomeSucceeded, want: 1},
		{name: "reported USD first", quantity: "100", reported: "0.001", outcome: usage.OutcomeSucceeded, want: 2},
		{name: "over ceiling", quantity: "100000000000000000", outcome: usage.OutcomeSucceeded, want: 15},
		{name: "provider failure", quantity: "10", outcome: usage.OutcomeFailed, cause: "provider", want: 2},
		{name: "service failure", quantity: "10", outcome: usage.OutcomeFailed, cause: "service", want: 2, compensation: 1},
		{name: "unknown", outcome: usage.OutcomeFailed, cause: "unknown"},
		{name: "measured zero", quantity: "0", outcome: usage.OutcomeCancelled},
		{name: "cancelled", quantity: "10", outcome: usage.OutcomeCancelled, want: 2},
		{name: "master reference", quantity: "10", outcome: usage.OutcomeFailed, cause: "service", master: true, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, handle, _ := unitFixture(t)
			b := unitBudget(t, "scope")
			tier := plan.Basic
			if tc.master {
				tier = plan.Master
			}
			s, q := unitStart(t, svc, "one", tier, b)
			if q.MaxCredits != 15 {
				t.Fatal(q.MaxCredits)
			}
			for range 2 {
				if err := svc.Hold(t.Context(), s); err != nil {
					t.Fatal(err)
				}
			}
			e := llm.SpeechEvidence{RequestID: "supplier-request", ReportedUSD: tc.reported}
			if tc.quantity != "" {
				e.Units = []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: tc.quantity}}
			}
			c := reportUnit(t, svc, "one", b, e)
			if err := svc.RecordUnitCall(t.Context(), c, e); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			failures := make(chan error, 8)
			for range 8 {
				wg.Go(func() { failures <- svc.SettleCause(t.Context(), "one", tc.outcome, tc.cause) })
			}
			wg.Wait()
			close(failures)
			for err := range failures {
				if err != nil {
					t.Fatal(err)
				}
			}
			a, err := svc.ReservationAccounting(t.Context(), "alice", "one")
			if err != nil || a == nil || !a.Settled {
				t.Fatal(a, err)
			}
			_, remaining := lotRemaining(t, handle, "original")
			expected := 200 - tc.want
			if tc.master {
				expected = 200
				if *a.ShadowCharge != tc.want || *a.FinalCharge != 0 {
					t.Fatal(a)
				}
			} else if *a.FinalCharge != tc.want {
				t.Fatal(a)
			}
			if remaining != expected {
				t.Fatal("wrong debit", remaining, expected)
			}
			var events, tokens, comp int
			var units, identity string
			if err := handle.Reader.QueryRow("SELECT COUNT(*),SUM(prompt_tokens+completion_tokens+reasoning_tokens) FROM usage_events WHERE job_id='one'").Scan(&events, &tokens); err != nil || events != 1 || tokens != 0 {
				t.Fatal(events, tokens, err)
			}
			if err := handle.Reader.QueryRow("SELECT evidence_json,supplier_request_id FROM usage_unit_events").Scan(&units, &identity); err != nil || identity != e.RequestID {
				t.Fatal(units, identity, err)
			}
			if tc.quantity != "" && !strings.Contains(units, tc.quantity) {
				t.Fatal("decimal evidence lost", units)
			}
			if err := handle.Reader.QueryRow("SELECT COALESCE(SUM(granted),0) FROM credit_lots WHERE kind='compensation'").Scan(&comp); err != nil || comp != tc.compensation {
				t.Fatal(comp, err)
			}
		})
	}
}

func TestSQLiteUnitCallClaimsSurviveRetryAndDedupeSupplierIdentity(t *testing.T) {
	svc, handle, _ := unitFixture(t)
	b := unitBudget(t, "one")
	b.Count = 2
	s, _ := unitStart(t, svc, "one", plan.Basic, b)
	if err := svc.Hold(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	e := llm.SpeechEvidence{RequestID: "same-supplier-id", ReportedUSD: "0.001"}
	input, _ := unitRequest().Input()
	var concurrent sync.WaitGroup
	results := make(chan error, 8)
	var accepted atomic.Int32
	for range 8 {
		concurrent.Go(func() {
			claim, err := svc.AdmitUnitCall(unitWork(t.Context(), "one", b), input)
			if err == nil {
				accepted.Add(1)
				err = svc.RecordUnitCall(t.Context(), claim, e)
			}
			results <- err
		})
	}
	concurrent.Wait()
	close(results)
	for err := range results {
		if err != nil && !errors.Is(err, usage.ErrUnitCall) {
			t.Fatal(err)
		}
	}
	if accepted.Load() != 2 {
		t.Fatal("concurrent claim limit", accepted.Load())
	}
	if _, err := svc.AdmitUnitCall(unitWork(t.Context(), "one", b), input); !errors.Is(err, usage.ErrUnitCall) {
		t.Fatal("extra retry allowed", err)
	}
	if err := svc.Settle(t.Context(), "one", usage.OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settle(t.Context(), "one", usage.OutcomeCancelled); err != nil {
		t.Fatal(err)
	}
	a, _ := svc.ReservationAccounting(t.Context(), "alice", "one")
	if a == nil || a.FinalCharge == nil || *a.FinalCharge != 2 || a.SettlementReason != "succeeded" {
		t.Fatal("supplier evidence charged twice", a)
	}
	var calls int
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_events WHERE job_id='one'").Scan(&calls); err != nil || calls != 2 {
		t.Fatal(calls, err)
	}
}

func TestSQLiteUnitExactJobRoundingAndOriginRefundAcrossExpiryReset(t *testing.T) {
	svc, handle, _ := unitFixture(t)
	one, two := unitBudget(t, "one"), unitBudget(t, "two")
	one.Tariffs[0].USDPerUnit = "0.000675675"
	two.Tariffs[0].USDPerUnit = "0.000675675"
	s, _ := unitStart(t, svc, "one", plan.Basic, one, two)
	if err := svc.Hold(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := handle.Writer.Exec("UPDATE credit_lots SET expires_at=? WHERE id='original'", expiry); err != nil {
		t.Fatal(err)
	}
	insertLot(t, handle, "new-window", "alice", "daily", 500, nil, time.Now())
	for i, b := range []usage.UnitBudget{one, two} {
		reportUnit(t, svc, "one", b, llm.SpeechEvidence{RequestID: string(rune('a' + i)), Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "0.5"}}})
	}
	if err := svc.Settle(t.Context(), "one", usage.OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	a, _ := svc.ReservationAccounting(t.Context(), "alice", "one")
	if *a.FinalCharge != 1 {
		t.Fatal("per-call rounding", a)
	}
	_, original := lotRemaining(t, handle, "original")
	_, fresh := lotRemaining(t, handle, "new-window")
	if original != 199 || fresh != 500 {
		t.Fatal("moved credit across periods", original, fresh)
	}
	var gotExpiry string
	if err := handle.Reader.QueryRow("SELECT expires_at FROM credit_lots WHERE id='original'").Scan(&gotExpiry); err != nil || gotExpiry != expiry {
		t.Fatal(gotExpiry, err)
	}
}

func TestSQLiteUnitEnqueueFailureReturnsWholeHoldAndDoesNotReplayQuote(t *testing.T) {
	svc, handle, _ := unitFixture(t)
	b := unitBudget(t, "scope")
	s, _ := unitStart(t, svc, "failed-enqueue", plan.Basic, b)
	if err := svc.Hold(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	if err := svc.Release(t.Context(), s.JobID); err != nil {
		t.Fatal(err)
	}
	_, remaining := lotRemaining(t, handle, "original")
	if remaining != 200 {
		t.Fatal(remaining)
	}
	if err := svc.Hold(t.Context(), s); err == nil {
		t.Fatal("consumed quote replayed after release")
	}
	var count int
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

type unitProvider struct {
	calls           atomic.Int32
	started, finish chan struct{}
	evidence        llm.SpeechEvidence
}

func (p *unitProvider) DesignVoice(ctx context.Context, _ llm.VoiceDesignRequest) (llm.VoiceDesignResponse, error) {
	p.calls.Add(1)
	if p.started != nil {
		close(p.started)
		<-p.finish
	}
	return llm.VoiceDesignResponse{Candidates: make([]llm.VoiceCandidate, 3), Evidence: p.evidence}, ctx.Err()
}
func (p *unitProvider) ConfirmVoice(context.Context, llm.VoiceConfirmationRequest) (llm.VoiceConfirmationResponse, error) {
	p.calls.Add(1)
	return llm.VoiceConfirmationResponse{}, nil
}
func (p *unitProvider) SynthesizeSpeech(context.Context, llm.SpeechRequest) (llm.SpeechResponse, error) {
	p.calls.Add(1)
	return llm.SpeechResponse{}, nil
}

func TestSpeechMeterRefusalsIssueZeroProviderCallsAndThreeAuditionsOneEvent(t *testing.T) {
	svc, handle, checker := unitFixture(t)
	b := unitBudget(t, "scope")
	s, _ := unitStart(t, svc, "one", plan.Basic, b)
	p := &unitProvider{evidence: llm.SpeechEvidence{RequestID: "design", Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "100"}}}}
	m := usage.SpeechMeter{Provider: p, Ledger: svc}
	if _, err := m.DesignVoice(t.Context(), unitRequest()); err == nil {
		t.Fatal("missing work accepted")
	}
	if _, err := m.DesignVoice(unitWork(t.Context(), "one", b), unitRequest()); err == nil {
		t.Fatal("missing admission accepted")
	}
	if err := svc.Hold(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	changed := unitRequest()
	changed.PreviewText += "!"
	if _, err := m.DesignVoice(unitWork(t.Context(), "one", b), changed); err == nil {
		t.Fatal("changed input accepted")
	}
	checker.unavailable.Store(true)
	if _, err := m.DesignVoice(unitWork(t.Context(), "one", b), unitRequest()); err == nil {
		t.Fatal("price drift accepted")
	}
	checker.unavailable.Store(false)
	if p.calls.Load() != 0 {
		t.Fatal("refusal called supplier", p.calls.Load())
	}
	out, err := m.DesignVoice(unitWork(t.Context(), "one", b), unitRequest())
	if err != nil || len(out.Candidates) != 3 {
		t.Fatal(out, err)
	}
	if _, err := m.DesignVoice(unitWork(t.Context(), "one", b), unitRequest()); err == nil {
		t.Fatal("automatic retry allowed")
	}
	var count int
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&count); err != nil || count != 1 || p.calls.Load() != 1 {
		t.Fatal(count, p.calls.Load(), err)
	}
}

func TestSpeechMeterCancellationRecordsLateEvidenceWithoutRetrospectiveDebit(t *testing.T) {
	svc, handle, _ := unitFixture(t)
	b := unitBudget(t, "scope")
	s, _ := unitStart(t, svc, "one", plan.Basic, b)
	if err := svc.Hold(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	p := &unitProvider{started: make(chan struct{}), finish: make(chan struct{}), evidence: llm.SpeechEvidence{RequestID: "inflight", ReportedUSD: "0.01"}}
	m := usage.SpeechMeter{Provider: p, Ledger: svc}
	ctx, cancel := context.WithCancel(unitWork(t.Context(), "one", b))
	finished := make(chan error, 1)
	go func() { _, err := m.DesignVoice(ctx, unitRequest()); finished <- err }()
	<-p.started
	cancel()
	if err := svc.Settle(t.Context(), "one", usage.OutcomeCancelled); err != nil {
		t.Fatal(err)
	}
	close(p.finish)
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := svc.Settle(t.Context(), "one", usage.OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	a, _ := svc.ReservationAccounting(t.Context(), "alice", "one")
	_, remaining := lotRemaining(t, handle, "original")
	if *a.FinalCharge != 0 || remaining != 200 || a.SettlementReason != "cancelled" {
		t.Fatal(a, remaining)
	}
	var count int
	if err := handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_unit_events WHERE reported_usd='0.01'").Scan(&count); err != nil || count != 1 {
		t.Fatal("lost cancelled evidence", count, err)
	}
}
