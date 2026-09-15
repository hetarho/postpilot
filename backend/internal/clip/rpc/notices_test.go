package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	"testing"
)

func TestProjectNoticeProjectionFiltersEditedCutAndKeepsStableCode(t *testing.T) {
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: "fp", EndMS: 15000}}}
	clip.AddPlanNotice(&plan, "plan_cut_rate", "cut", "", "repair")
	clip.AddPlanNotice(&plan, "plan_target_duration", "", "", "shortfall")
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := projectProto(clip.Project{EditPlan: raw})
	if len(got.Notices) != 2 || got.Notices[0].Code != "plan_cut_rate" || got.Notices[0].CutId != "cut" || got.Notices[0].Action != "repair" {
		t.Fatal(got.Notices)
	}
	plan.NoticeCutRevisions = map[string]int{"cut": 1}
	raw, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	got = projectProto(clip.Project{EditPlan: raw})
	if len(got.Notices) != 1 || got.Notices[0].CutId != "" {
		t.Fatal("edited cut notice escaped the RPC filter", got.Notices)
	}
}
