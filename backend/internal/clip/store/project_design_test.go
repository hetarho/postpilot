package store_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func designedBody(intro, outro string) string {
	return `<clip version="1" intro="` + intro + `" caption="bold" outro="` + outro + `">` +
		`<text id="intro" kind="fixed" role="hook"/>` +
		`<text id="outro" kind="fixed" role="ending"/></clip>`
}

// The design selection is the PROJECT's alone (CLIP-14, CLIP-139, CLIP-142):
// every project starts at the shared defaults, template or no template, and may
// change every one of them.
func TestDesignSelectionStartsAtTheDefaultsAndIsOwnedByTheProject(t *testing.T) {
	service, _, _ := setup(t)
	template, err := service.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "디자인", Preset: "stay", Accent: "teal", CompositionBody: designedBody("a", "b")})
	if err != nil {
		t.Fatal(err)
	}
	p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "여행", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "sponsored"})
	if err != nil {
		t.Fatal(err)
	}
	// The template's body still names presets and the project takes none of
	// them: an empty selection IS the shared default (CLIP-14).
	if p.IntroPreset != "" || p.OutroPreset != "" || len(p.CaptionStyles) != 0 {
		t.Fatal("a template seeded the project's design", p.IntroPreset, p.OutroPreset, p.CaptionStyles)
	}
	if presets := p.DesignSelection().RegionPresets(); presets.Intro != "b" || presets.Outro != "e" {
		t.Fatal("the unset selection did not resolve to the shared defaults", presets)
	}
	if resolved := clip.ResolvedCaptionStyles(p.CaptionStyles); len(resolved) != 1 || resolved[0] != "bold" {
		t.Fatal("the unset styles did not resolve to the default style", resolved)
	}
	intro, outro, styles := "b", "e", []string{}
	changed, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroPreset: &intro, OutroPreset: &outro, CaptionStyles: &styles})
	if err != nil || changed.IntroPreset != "b" || changed.OutroPreset != "e" || len(changed.CaptionStyles) != 0 {
		t.Fatal("the owner's selection was not saved", changed.IntroPreset, changed.OutroPreset, changed.CaptionStyles, err)
	}
	// An empty selection is a real value, and it reads as the default style
	// alone rather than as no captions at all (CDS-25).
	if resolved := clip.ResolvedCaptionStyles(changed.CaptionStyles); len(resolved) != 1 || resolved[0] != "bold" {
		t.Fatal("an empty selection did not resolve to the default style", resolved)
	}
	read, err := service.GetProject(t.Context(), "alice", p.ID)
	if err != nil || read.IntroPreset != "b" || read.OutroPreset != "e" || len(read.CaptionStyles) != 0 {
		t.Fatal("the selection did not survive a read", read.IntroPreset, read.OutroPreset, read.CaptionStyles, err)
	}
	invented := "z"
	for _, patch := range []clip.ProjectPatch{{IntroPreset: &invented}, {OutroPreset: &invented}, {CaptionStyles: &[]string{"sparkle"}}, {CaptionStyles: &[]string{"bold", "bold"}}} {
		if _, err := service.UpdateProject(t.Context(), "alice", p.ID, patch); err == nil {
			t.Fatal("a selection outside the approved set was accepted", patch)
		}
	}
}

// Changing the selection re-renders the SAME plan: the result goes stale, and no
// observation and no writing call is repaid (CLIP-139).
func TestChangingTheDesignSelectionStalesTheResultWithoutRewritingThePlan(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || before.Result == nil || before.EditPlanRevision != before.RenderedPlanRevision {
		t.Fatal("the fixture has no rendered result to stale", err)
	}
	intro := "a"
	after, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{IntroPreset: &intro})
	if err != nil {
		t.Fatal(err)
	}
	if after.EditPlanRevision != before.EditPlanRevision+1 || after.RenderedPlanRevision != before.RenderedPlanRevision {
		t.Fatal("the result did not go stale", after.EditPlanRevision, after.RenderedPlanRevision)
	}
	if after.EditPlan != before.EditPlan || after.Analysis != before.Analysis {
		t.Fatal("the plan or the observations were rewritten by a render-only change")
	}
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || !q.Pricing.RenderOnly() || q.Pricing.MaxCredits != 0 {
		t.Fatalf("a preset change repriced the writing: %+v %v", q.Pricing, err)
	}
	same, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{IntroPreset: &intro})
	if err != nil || same.EditPlanRevision != after.EditPlanRevision {
		t.Fatal("an unchanged value still staled the result", same.EditPlanRevision, err)
	}
	// The styles are the same kind of choice: selecting one is a change the
	// render has to be redone for, even where the default it replaces draws the
	// same caption.
	styles := []string{"keynote"}
	restyled, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{CaptionStyles: &styles})
	if err != nil || restyled.EditPlanRevision != after.EditPlanRevision+1 {
		t.Fatal("changing the allowed styles did not stale the result", restyled.EditPlanRevision, err)
	}
}
