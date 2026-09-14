package store_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

const ownerCutID = "owner-3f2504e0-4f89-41d3-9a0c-0305e82c3301"

// layoutRenderer is the composition executor the editor needs: it measures a
// validated plan and returns it unchanged, so the test exercises the save path
// rather than the layout engine.
type layoutRenderer struct{ *rendererFake }

func (layoutRenderer) CompositionPlanVersion() int { return clip.CompositionPlanVersion }
func (layoutRenderer) LayoutComposition(_ context.Context, p clip.EditPlan, _ []clip.RenderSource) (clip.EditPlan, []clip.CompositionElement, error) {
	return p, nil, nil
}

// The whole path an editor uses: one optimistic full-plan save that adds
// footage, splits a cut and reorders, through the existing revision check.
func TestOwnerCutCreationRidesTheExistingOptimisticSave(t *testing.T) {
	h, _, _ := completedClip(t)
	ctx := context.Background()
	// The editor only exists where planning and rendering both understand the
	// current plan, so the fixture's executors advertise that capability.
	h.service = clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, compositionPlanner{h.planner}, layoutRenderer{h.renderer}, generationJobs{h.queue}, h.cfg).WithFinisher(generationFinisher{h.store}).WithCredits(&quotePricing{}, nil)
	h.projects.SetGeneration(h.service)
	p, err := h.projects.GetProject(ctx, "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := h.service.EditingState(p)
	if err != nil {
		t.Fatal(err)
	}
	draft := state.Plan
	if !draft.NativeComposition || len(draft.Cuts) != 1 {
		t.Fatal("the fixture is not a single native cut", draft)
	}
	origin := draft.Cuts[0]
	// A second look at the same observed scene, after the selected range.
	draft.Cuts = append(draft.Cuts, clip.CorrectionCut{ID: ownerCutID, SourceID: origin.SourceID, Fingerprint: origin.Fingerprint,
		StartMS: 40000, EndMS: 55000, Creation: &clip.CutCreation{Kind: clip.CutAdd, OriginID: origin.ID}})
	draft.DurationMS = 45000
	next, err := h.service.SaveCorrection(ctx, "alice", h.project.ID, p.EditPlanRevision, draft)
	if err != nil {
		t.Fatal("an owner add was refused", err)
	}
	if next.EditPlanRevision != p.EditPlanRevision+1 {
		t.Fatal("the save did not advance the revision once", next.EditPlanRevision)
	}
	if strings.Contains(next.EditPlan, "Creation") {
		t.Fatal("creation metadata was stored")
	}
	saved, _, err := clip.DecodeEditPlan(next.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Cuts) != 2 || saved.Cuts[1].ID != ownerCutID || saved.Cuts[1].Rate() != clip.RateUnitPermille {
		t.Fatal("the created cut was not stored as approved footage", saved.Cuts)
	}
	// Replaying the same request against the new revision is refused, so an id
	// cannot be created twice by a retry.
	if _, err := h.service.SaveCorrection(ctx, "alice", h.project.ID, next.EditPlanRevision, draft); err == nil {
		t.Fatal("a replayed creation was accepted")
	}
	// Reorder and split in one save, on the plan as it now stands.
	current, err := h.service.EditingState(next)
	if err != nil {
		t.Fatal(err)
	}
	second := current.Plan
	second.Cuts[0].EndMS = 12000
	second.Cuts = slices.Insert(second.Cuts, 1, clip.CorrectionCut{ID: "owner-6ba7b810-9dad-41d1-80b4-00c04fd430c8",
		StartMS: 12000, EndMS: 30000, Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: second.Cuts[0].ID}})
	second.Cuts[0], second.Cuts[2] = second.Cuts[2], second.Cuts[0]
	second.Cuts[0].TransitionMS = 0
	after, err := h.service.SaveCorrection(ctx, "alice", h.project.ID, next.EditPlanRevision, second)
	if err != nil {
		t.Fatal("a split beside a reorder was refused", err)
	}
	split, _, err := clip.DecodeEditPlan(after.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	if len(split.Cuts) != 3 {
		t.Fatal("the split did not produce a third cut", split.Cuts)
	}
	// Two cuts of one source that touch are adjacent, never an overlap.
	if len(clip.SourceOverlaps(split.Cuts)) != 0 {
		t.Fatal("same-source adjacency was read as overlap")
	}
	// A foreign owner cannot reach the same plan at all.
	if _, err := h.service.SaveCorrection(ctx, "bob", h.project.ID, after.EditPlanRevision, second); err == nil {
		t.Fatal("a foreign owner saved a created cut")
	}
}
