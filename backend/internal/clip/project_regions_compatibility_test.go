package clip_test

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"testing"
)

func TestSavedPlanRegionsRemainTheCompatibilityAuthority(t *testing.T) {
	p, plan := creationFixture(t)
	p.IntroPreset, p.OutroPreset = "serif", "credits"
	plan.Portable.Elements = nil
	var err error
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = clip.DecodeEditPlan(p.EditPlan); err != nil {
		t.Fatal("invalid fixture", err)
	}
	r := clip.EffectiveProjectRegions(p)
	if r.Intro.Enabled || r.Outro.Enabled {
		t.Fatal("empty saved plan gained regions", r)
	}
	plan.Portable.Elements = []clip.PortableText{{Resolved: composition.ResolvedElement{InstanceID: "saved-intro", Element: composition.Element{ID: "saved-intro", Kind: "ai", Role: "hook", Basis: "output-start"}, Text: "Saved words", StartMS: 0, EndMS: 2500}}}
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = clip.DecodeEditPlan(p.EditPlan); err != nil {
		t.Fatal("invalid region fixture", err)
	}
	r = clip.EffectiveProjectRegions(p)
	if !r.Intro.Enabled || r.Outro.Enabled || r.Intro.Slots[0].Text != "Saved words" || r.Intro.Slots[0].OwnerFixed {
		t.Fatal("saved generated words lost", r)
	}
}
