package store_test

import (
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func designedBody(intro, outro string) string {
	return `<clip version="1" intro="` + intro + `" caption="bold" outro="` + outro + `">` +
		`<text id="intro" kind="fixed" role="hook"/>` +
		`<text id="outro" kind="fixed" role="ending"/></clip>`
}

// The design selection is the PROJECT's alone (CLIP-14, CLIP-111, CLIP-139,
// CLIP-142): every new project stores intro A and outro B, template or no
// template, and may change every one of them.
func TestDesignSelectionStartsAtTheDefaultsAndIsOwnedByTheProject(t *testing.T) {
	service, _, _ := setup(t)
	template, err := service.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "디자인", CompositionBody: designedBody("b", "e")})
	if err != nil {
		t.Fatal(err)
	}
	p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "여행", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "sponsored"})
	if err != nil {
		t.Fatal(err)
	}
	// The template's body names other presets and the project takes none of
	// them: it stores the new-project defaults as ids, so a later change of
	// defaults never restyles it (CLIP-14, CLIP-111).
	if p.IntroPreset != "a" || p.OutroPreset != "b" || len(p.CaptionStyles) != 0 {
		t.Fatal("a template seeded the project's design", p.IntroPreset, p.OutroPreset, p.CaptionStyles)
	}
	// An empty id stored before the defaults moved keeps rendering what it
	// always rendered.
	if presets := (clip.ProjectDesign{}).RegionPresets(); presets.Intro != "b" || presets.Outro != "e" {
		t.Fatal("an unset selection changed its look", presets)
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
	// The generation stops at the plan (CLIP-151); the render the selection
	// stales is the one the owner starts.
	h.render(t)
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || before.Result == nil || before.EditPlanRevision != before.RenderedPlanRevision {
		t.Fatal("the fixture has no rendered result to stale", err)
	}
	kept, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || !kept.Pricing.RenderOnly() || kept.Pricing.MaxCredits != 0 {
		t.Fatalf("the kept plan was not quoted as a render: %+v %v", kept.Pricing, err)
	}
	intro := "cover"
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
	// Choosing the preset switched the intro on, and an enabled region's slots are
	// what the writing calls draft (CLIP-111, CLIP-187): the kept plan no longer
	// answers them, so a generation is priced to write again.
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || q.Pricing.RenderOnly() || q.Pricing.MaxCredits == 0 {
		t.Fatalf("an intro to draft was quoted as a render: %+v %v", q.Pricing, err)
	}
	same, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{IntroPreset: &intro})
	if err != nil || same.EditPlanRevision != after.EditPlanRevision {
		t.Fatal("an unchanged value still staled the result", same.EditPlanRevision, err)
	}
	// The styles stale a render where a caption takes the selection's first
	// entry — this fixture's narration names no style ("auto") — so a new first
	// entry redraws it.
	styles := []string{"keynote"}
	restyled, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{CaptionStyles: &styles})
	if err != nil || restyled.EditPlanRevision != after.EditPlanRevision+1 {
		t.Fatal("changing the allowed styles did not stale the result", restyled.EditPlanRevision, err)
	}
	// A caption with a style of its own is not the selection's to redraw
	// (CLIP-142, CLIP-191): once the owner gives it one outside the selection,
	// changing the selection again leaves a render of it current.
	plan, err := clip.DecodeEditPlan(restyled.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	draft := clip.CorrectionFromPlan(plan)
	for i := range draft.Elements {
		if draft.Elements[i].Role == "caption" {
			draft.Elements[i].Owner.Style = "film"
		}
	}
	calls := h.planner.observe + h.planner.plans + h.planner.flows + h.planner.narrations + len(h.planner.revisions)
	if _, err := h.service.SaveCorrection(t.Context(), "alice", h.project.ID, restyled.EditPlanRevision, draft); err != nil {
		t.Fatal("an owner style outside the selection was refused:", err)
	}
	owned, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || !slices.Equal(owned.CaptionStyles, styles) {
		t.Fatal("the owner's choice widened the AI selection", owned.CaptionStyles, err)
	}
	// The style edit is a plan edit like any other (CLIP-191): a new revision, the
	// earlier render kept as it was and left stale, and no writing call, even
	// for a sequence style dearer than the AI estimate (CLIP-20, CLIP-145).
	if owned.EditPlanRevision != restyled.EditPlanRevision+1 || owned.RenderedPlanRevision != restyled.RenderedPlanRevision || owned.Result == nil || owned.Result.Key != before.Result.Key {
		t.Fatal("the style edit did not stale the kept render", owned.EditPlanRevision, owned.RenderedPlanRevision)
	}
	if h.planner.observe+h.planner.plans+h.planner.flows+h.planner.narrations+len(h.planner.revisions) != calls {
		t.Fatal("a style edit called a model")
	}
	narrowed := []string{"neon"}
	unstaled, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{CaptionStyles: &narrowed})
	if err != nil || unstaled.EditPlanRevision != owned.EditPlanRevision || unstaled.RenderedPlanRevision != owned.RenderedPlanRevision {
		t.Fatal("a selection change moved the revision of a plan it does not redraw", unstaled.EditPlanRevision, owned.EditPlanRevision, err)
	}
	if unstaled.EditPlan != owned.EditPlan {
		t.Fatal("a selection change rewrote the plan")
	}
}
