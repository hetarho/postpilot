package usage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type unavailableFreePath struct{ fakeModels }

func (unavailableFreePath) QualifyFree(context.Context, string, llm.FreePath) (bool, error) {
	return false, nil
}

func TestMissingFreeCapabilityRefusesBeforeAdmission(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "zero-price-with-unknown-capability"}
	store := newFakeStore()
	models := unavailableFreePath{fakeModels{ref: {Ref: ref, Stages: []string{"observe"},
		Levels: map[string]string{"observe": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"}}}
	svc := NewService(store, models, maxCompletion, fakeAnchors{anchor: testAnchor}).WithModelGrades()
	svc.now = func() time.Time { return seoulNoon }
	err := svc.Hold(context.Background(), Start{UserID: "alice", Plan: plan.Light, Kind: "generate", JobID: "unknown-free",
		Calls: []PlannedCall{{Ref: ref, Stage: "observe", Count: 1}}})
	var unavailable *FreePathError
	if !errors.As(err, &unavailable) || len(store.admissions) != 0 || store.spendCalls != 0 {
		t.Fatalf("missing capability admitted or spent work: err=%v admissions=%d spends=%d", err, len(store.admissions), store.spendCalls)
	}
}

type lightRateSource struct{}

func (lightRateSource) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 14_000_000, true, nil
}

type lightRateCache struct{ days map[string]RateDay }

func (c *lightRateCache) RateDay(_ context.Context, date string) (RateDay, bool, error) {
	day, ok := c.days[date]
	return day, ok, nil
}
func (c *lightRateCache) RecordRateDay(_ context.Context, day RateDay) error {
	c.days[day.Date] = day
	return nil
}
func (c *lightRateCache) LatestRateDay(_ context.Context, date string) (RateDay, bool, error) {
	day, ok := c.days[date]
	return day, ok, nil
}

func TestLightDailyGrantPaysOneBoundedCallButNotAnOversizedJob(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "priced-value-fixture"}
	models := fakeModels{ref: {Ref: ref, Stages: []string{"write"},
		Levels: map[string]string{"write": "value"}, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.3"}}
	store := newFakeStore()
	svc := NewService(store, models, 10_000, fakeAnchors{anchor: testAnchor}).WithModelGrades().WithRateSelector(
		NewRateSelector(lightRateSource{}, &lightRateCache{days: map[string]RateDay{}}))
	svc.now = func() time.Time { return seoulNoon }
	ctx := context.Background()
	call := PlannedCall{Ref: ref, Stage: "write", Count: 1, CompletionTokens: 4_000}
	first := Start{UserID: "alice", Plan: plan.Light, Kind: "write", JobID: "one-bounded-call", Calls: []PlannedCall{call}}
	if err := svc.Hold(ctx, first); err != nil {
		t.Fatal(err)
	}
	if len(store.admissions) != 1 || store.admissions[0].HoldCredits <= 0 || store.admissions[0].HoldCredits > 15 {
		t.Fatalf("Light's 15-credit daily grant did not bound one call: %+v", store.admissions)
	}
	second := first
	second.JobID = "too-many-calls"
	second.Calls = []PlannedCall{{Ref: ref, Stage: "write", Count: 100, CompletionTokens: 4_000}}
	var shortage *plan.InsufficientCreditsError
	if err := svc.Hold(ctx, second); !errors.As(err, &shortage) || len(store.admissions) != 1 {
		t.Fatalf("oversized hold bypassed reservation: err=%v admissions=%d", err, len(store.admissions))
	}
}

func TestModelGradesFreezeRightsAndKeepFreeAtZeroBalance(t *testing.T) {
	freeRef := llm.ModelRef{ProviderID: "openrouter", ModelID: "free"}
	paidRef := llm.ModelRef{ProviderID: "openrouter", ModelID: "paid"}
	models := fakeModels{
		freeRef: {Ref: freeRef, Stages: []string{"write"}, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"},
		paidRef: {Ref: paidRef, Stages: []string{"write"}, Levels: map[string]string{"write": "value"}, InputUSDPerMillion: "1", OutputUSDPerMillion: "1"},
	}
	store := newFakeStore()
	svc := NewService(store, models, maxCompletion, fakeAnchors{anchor: testAnchor}).WithRateSelector(&RateSelector{}).WithModelGrades()
	svc.now = func() time.Time { return seoulNoon }
	ctx := context.Background()
	start := Start{UserID: "alice", Plan: plan.Free, Kind: "write", JobID: "free-job", Calls: []PlannedCall{{Ref: freeRef, Stage: "write", Count: 1}}}
	if err := svc.Hold(ctx, start); err != nil {
		t.Fatal(err)
	}
	admission, found, err := svc.AdmissionForJob(ctx, start.JobID)
	if err != nil || !found || admission.HoldCredits != 0 || admission.AdmittedPlan != plan.Free || len(admission.AdmittedModels) != 1 || admission.AdmittedModels[0].Grade != "free" {
		t.Fatalf("free admission = %+v, found %v, error %v", admission, found, err)
	}
	start.JobID = "paid-job"
	start.Calls = []PlannedCall{{Ref: paidRef, Stage: "write", Count: 1}}
	var locked *ModelGradeError
	if err := svc.Hold(ctx, start); !errors.As(err, &locked) || locked.Required != plan.Light {
		t.Fatalf("free paid refusal = %v", err)
	}
	start.Calls = append(start.Calls, PlannedCall{Ref: freeRef, Stage: "write", Count: 1})
	if err := svc.Hold(ctx, start); !errors.As(err, &locked) {
		t.Fatalf("mixed free/paid job was admitted: %v", err)
	}
}
