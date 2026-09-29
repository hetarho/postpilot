package usage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

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
