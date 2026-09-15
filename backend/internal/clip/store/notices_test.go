package store_test

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestStoredNoticesSurviveReloadAndRerenderRecomputesPlanScope(t *testing.T) {
	h, p, _ := completedClip(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	clip.AddPlanNotice(&plan, "plan_cut_rate", plan.Cuts[0].ID, "", "repair")
	// Stale plan-level notice must be recomputed from the saved target on render.
	clip.AddPlanNotice(&plan, "plan_target_duration", "", "", "shortfall")
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := h.store.SaveCorrection(t.Context(), "alice", p.ID, p.EditPlanRevision, raw)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := clip.DecodeEditPlan(saved.EditPlan)
	if err != nil || len(clip.ActivePlanNotices(stored)) != 2 {
		t.Fatal("stored notices lost", err)
	}
	result := *p.Result
	result.CreatedAt = time.Now()
	if err = h.store.SaveRender(t.Context(), "alice", p.ID, saved.EditPlanRevision, result); err != nil {
		t.Fatal(err)
	}
	reloaded, err := h.projects.GetProject(t.Context(), "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := clip.DecodeEditPlan(reloaded.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	want := 1
	if after.DurationMS < reloaded.TargetDurationMS {
		want++
	}
	if len(clip.ActivePlanNotices(after)) != want || reloaded.EditPlanRevision != saved.EditPlanRevision || reloaded.RenderedPlanRevision != saved.EditPlanRevision {
		t.Fatal("rerender changed owner revision or did not recompute notices", clip.ActivePlanNotices(after))
	}
}
