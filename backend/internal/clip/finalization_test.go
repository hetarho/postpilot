package clip_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
	"testing"
	"time"
)

func TestNativeFinalizationUsesSavedEvidenceWithoutPixelsOrLayout(t *testing.T) {
	p, plan, styles := nativeHistoryFixture(t)
	p.UserID, p.ID, p.EditPlanRevision, p.RenderedPlanRevision = "owner", "project", 2, 2
	p.Result = &clip.Result{ID: "result", Key: "clip-results/result.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: plan.DurationMS, CreatedAt: time.Now()}
	req := clip.FinalizationRequest{UserID: p.UserID, ProjectID: p.ID, ExpectedRevision: 2, ExpectedResultID: "result"}
	if err := clip.ValidateFinalization(p, req, config.ClipRender(&config.Config{})); err != nil {
		t.Fatal(err)
	}
	plan.Portable.Elements[0].StaleEvidence = true
	var err error
	p.EditPlan, err = clip.EncodeEditPlan(plan, styles)
	if err != nil {
		t.Fatal(err)
	}
	if err := clip.ValidateFinalization(p, req, config.ClipRender(&config.Config{})); !errors.Is(err, clip.ErrFinalizationInvalid) {
		t.Fatal("stale evidence finalized", err)
	}
}
