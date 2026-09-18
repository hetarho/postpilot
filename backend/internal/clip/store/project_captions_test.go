package store_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// The pace and the accent are the PROJECT's alone: every clip starts unset,
// which is the shared default, and changing either re-renders the same plan
// rather than rewriting it (CLIP-14, CLIP-139).
func TestCaptionPaceAndAccentStartUnsetAndAreOwnedByTheProject(t *testing.T) {
	h := generationSetup(t)
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.CaptionPace != "" || p.Accent != "" {
		t.Fatal("a template seeded the project's pace or accent", p.CaptionPace, p.Accent, h.template.CaptionPace, h.template.Accent)
	}
	pace, accent := "rapid", "teal"
	changed, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CaptionPace: &pace, Accent: &accent})
	if err != nil || changed.CaptionPace != "rapid" || changed.Accent != "teal" {
		t.Fatal("the owner's choice was not saved", changed.CaptionPace, changed.Accent, err)
	}
	// Both are clearable: an empty value is the steady pace and no accent.
	empty := ""
	cleared, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CaptionPace: &empty, Accent: &empty})
	if err != nil || cleared.CaptionPace != "" || cleared.Accent != "" {
		t.Fatal("the choice could not be cleared", cleared.CaptionPace, cleared.Accent, err)
	}
	invented := "hurried"
	if _, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CaptionPace: &invented}); err == nil {
		t.Fatal("an unsupported pace was accepted")
	}
	if _, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Accent: &invented}); err == nil {
		t.Fatal("a colour outside the palette was accepted")
	}
}

func TestChangingThePaceStalesTheResultWithoutRewritingThePlan(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	// The generation stops at the plan (CLIP-151), so the render this staling
	// is about is the one the owner starts next.
	h.render(t)
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || before.Result == nil || before.EditPlanRevision != before.RenderedPlanRevision {
		t.Fatal("the fixture has no rendered result to stale", err)
	}
	pace := "rapid"
	after, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{CaptionPace: &pace})
	if err != nil {
		t.Fatal(err)
	}
	if after.EditPlanRevision != before.EditPlanRevision+1 || after.RenderedPlanRevision != before.RenderedPlanRevision {
		t.Fatal("the result did not go stale", after.EditPlanRevision, after.RenderedPlanRevision)
	}
	if after.EditPlan != before.EditPlan || after.Analysis != before.Analysis {
		t.Fatal("the plan or the observations were rewritten by a render-only change")
	}
	// The quote still resumes at rendering: no writing call and no observation
	// is owed for a pace change.
	q, err := h.service.Quote(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || !q.Pricing.RenderOnly() || q.Pricing.MaxCredits != 0 {
		t.Fatalf("a pace change repriced the writing: %+v %v", q.Pricing, err)
	}
	// Saving the same value again changes nothing at all.
	same, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{CaptionPace: &pace})
	if err != nil || same.EditPlanRevision != after.EditPlanRevision {
		t.Fatal("an unchanged value still staled the result", same.EditPlanRevision, err)
	}
}
