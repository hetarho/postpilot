package store_test

import (
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func boundedSpeechBudget(t *testing.T) (usage.UnitBudget, llm.SpeechRequest) {
	request := llm.SpeechRequest{Model: llm.ModelRef{ProviderID: "p", ModelID: "speech"}, Voice: "owned-confirmed", Text: strings.Repeat("가", 500), Settings: llm.SpeechSettings{Speed: 1}}
	input, e := request.Input()
	if e != nil {
		t.Fatal(e)
	}
	b := unitBudget(t, "bounded")
	b.Ref = input.Ref
	b.Operation = "speech"
	b.InputDigest = input.Digest
	b.InputCharacters = 500
	b.AuxiliaryCharacters = 0
	b.ParametersDigest = input.ParametersDigest
	b.Count = 32
	b.BoundedInput = true
	b.TotalInputCharacters = 2000
	b.InputIdentityDigest = input.IdentityDigest
	return b, request
}
func TestSQLiteBoundedSpeechAtomicallyEnforcesCallsAndAggregateCharacters(t *testing.T) {
	svc, _, _ := unitFixture(t)
	b, request := boundedSpeechBudget(t)
	usd, e := b.MaximumUSD()
	if e != nil || usd.RatString() != "1/5" {
		t.Fatal("quoted count times per-call characters instead of corpus", usd, e)
	}
	start, _ := unitStart(t, svc, "bounded", plan.Master, b)
	if e = svc.Hold(t.Context(), start); e != nil {
		t.Fatal(e)
	}
	var group sync.WaitGroup
	var allowed atomic.Int32
	for range 12 {
		group.Go(func() {
			input, _ := request.Input()
			if _, e := svc.AdmitUnitCall(unitWork(t.Context(), "bounded", b), input); e == nil {
				allowed.Add(1)
			}
		})
	}
	group.Wait()
	if allowed.Load() != 4 {
		t.Fatal("total character ceiling not atomic", allowed.Load())
	}
	request.Voice = "another"
	input, _ := request.Input()
	if _, e = svc.AdmitUnitCall(unitWork(t.Context(), "bounded", b), input); e == nil {
		t.Fatal("changed private identity admitted")
	}
	svc, _, _ = unitFixture(t)
	b, request = boundedSpeechBudget(t)
	b.TotalInputCharacters = 100
	b.Count = 2
	start, _ = unitStart(t, svc, "count", plan.Master, b)
	if e = svc.Hold(t.Context(), start); e != nil {
		t.Fatal(e)
	}
	request.Text = "한"
	input, _ = request.Input()
	for i := range 3 {
		_, e = svc.AdmitUnitCall(unitWork(t.Context(), "count", b), input)
		if (i < 2) != (e == nil) {
			t.Fatal("call ceiling", i, e)
		}
	}
}
func TestSQLiteMixedApprovalUsesOneHoldAndOneCombinedRounding(t *testing.T) {
	svc, handle, _ := unitFixture(t)
	b, _ := boundedSpeechBudget(t)
	start, q := unitStart(t, svc, "mixed", plan.Master, b)
	policy := llm.CallPolicy{Ref: pricedRef, Stage: "write", CompletionTokens: 32768, InputUSDPerMillion: "0.15", OutputUSDPerMillion: "0"}
	token := []usage.PricedCall{{Policy: policy, Count: 2}}
	cap, e := usage.MixedReservationCredits(token, []usage.UnitBudget{b}, q.Rate)
	if e != nil {
		t.Fatal(e)
	}
	start.Approval.Calls = token
	start.Approval.ApprovedMaxCredits = cap
	start.Calls = append(start.Calls, usage.PlannedCall{Ref: pricedRef, Stage: "write", CompletionTokens: 32768, Count: 2})
	if e = svc.Hold(t.Context(), start); e != nil {
		t.Fatal(e)
	}
	if e = svc.Hold(t.Context(), start); e != nil {
		t.Fatal("matching replay", e)
	}
	var n int
	if e = handle.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions WHERE job_id='mixed'").Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	// Lowering the exact maximum is refused before a second job is admitted.
	start.JobID = "changed"
	start.Approval.ApprovedMaxCredits = cap - 1
	if e = svc.Hold(t.Context(), start); e == nil {
		t.Fatal("lowered ceiling accepted")
	}
}
