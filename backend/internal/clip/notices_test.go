package clip_test

import (
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestNoticesRetainStorageAndClearOnlyEditedTargets(t *testing.T) {
	p, plan := nativeHistoryFixture(t)
	text := &plan.Portable.Elements[0]
	text.FallbackReason = "shorter_copy"
	plan.Portable.Fallbacks = []clip.CopyFallback{{ElementID: text.Resolved.Element.ID, CutID: text.Resolved.CutID, Reason: "shorter_copy"}}
	clip.AddPlanNotice(&plan, "plan_focal", plan.Cuts[0].ID, "", "repair")
	clip.AddPlanNotice(&plan, "composition_section_order", "removed-cut", "", "removal")
	clip.RecomputePlanNotices(&plan, 45000, 0)
	original := clip.ActivePlanNotices(plan, composition.DefaultDesign())
	if len(original) != 4 {
		t.Fatal(original)
	}
	p.EditPlan, _ = clip.EncodeEditPlan(plan)
	noOp, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, clip.CorrectionFromPlan(plan))
	if err != nil || !reflect.DeepEqual(clip.ActivePlanNotices(noOp, composition.DefaultDesign()), original) {
		t.Fatal("no-op cleared a notice", err)
	}
	draft := clip.CorrectionFromPlan(plan)
	draft.Cuts[0].Focal = &clip.Point{X: .1, Y: .2}
	draft.Elements[0].Text = "직접 쓴 문구"
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if len(clip.ActivePlanNotices(next, composition.DefaultDesign())) != 2 || len(next.Notices) != len(plan.Notices) || len(next.Portable.Fallbacks) != 1 {
		t.Fatal("edits did not filter targets without deleting records", clip.ActivePlanNotices(next, composition.DefaultDesign()))
	}
	encoded, err := clip.EncodeEditPlan(next)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := clip.DecodeEditPlan(encoded)
	if err != nil || !reflect.DeepEqual(clip.ActivePlanNotices(next, composition.DefaultDesign()), clip.ActivePlanNotices(restored, composition.DefaultDesign())) {
		t.Fatal("owner edit filtering lost on reload", err)
	}
	clip.RecomputePlanNotices(&restored, restored.DurationMS, 0)
	if len(clip.ActivePlanNotices(restored, composition.DefaultDesign())) != 1 {
		t.Fatal("rerender kept resolved plan-level notice")
	}
}

func TestLegacyCorrectionKeepsNoticesUntilTheCutChanges(t *testing.T) {
	p, _ := correctionFixture(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	clip.AddPlanNotice(&plan, "plan_cut_rate", plan.Cuts[0].ID, "", "repair")
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	draft := clip.CorrectionFromPlan(plan)
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil || len(clip.ActivePlanNotices(next, composition.DefaultDesign())) != 1 {
		t.Fatal("no-op lost history", err)
	}
	draft.Cuts[0].VolumePermille = 500
	next, err = clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil || len(clip.ActivePlanNotices(next, composition.DefaultDesign())) != 0 || len(next.Notices) != 1 {
		t.Fatal("edit did not filter stored history", err)
	}
}
