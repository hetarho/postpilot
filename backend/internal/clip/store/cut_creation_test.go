package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
	saved, err := clip.DecodeEditPlan(next.EditPlan)
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
	split, err := clip.DecodeEditPlan(after.EditPlan)
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

// Rejected owner drafts stay byte-for-byte available to the editor, and no
// failed validation changes the saved assembly, prior result or provider count.
func TestAssemblyRejectionsPreserveDraftAndPreviousResult(t *testing.T) {
	for _, kind := range []string{"unsupported-rate", "insufficient-cadence", "overlap", "authored-interval"} {
		t.Run(kind, func(t *testing.T) {
			h := generationSetup(t)
			h.service = clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, ownedPlanWriter{h.planner}, ownedPlanRenderer{h.renderer}, generationJobs{h.queue}, h.cfg).WithFinisher(generationFinisher{h.store}).WithCredits(&quotePricing{}, nil)
			h.projects.SetGeneration(h.service)
			template, err := legacyTemplate(t, h.store, "alice", "assembly-rejections", `<clip version="1"><repeat for="scenes"><scene id="shot"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></repeat><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`), error(nil)
			if err != nil {
				t.Fatal(err)
			}
			h.project, err = h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{VideoTemplateID: &template.ID})
			if err != nil {
				t.Fatal(err)
			}
			h.start(t)
			if err := h.run(t); err != nil {
				t.Fatal(err)
			}
			h.service = clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, compositionPlanner{h.planner}, layoutRenderer{h.renderer}, generationJobs{h.queue}, h.cfg).WithFinisher(generationFinisher{h.store}).WithCredits(&quotePricing{}, nil)
			h.projects.SetGeneration(h.service)
			p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
			if err != nil {
				t.Fatal(err)
			}
			state, err := h.service.EditingState(p)
			if err != nil {
				t.Fatal(err)
			}
			draft := state.Plan
			switch kind {
			case "unsupported-rate":
				draft.Cuts[0].PlaybackRatePermille = 1100
			case "insufficient-cadence":
				draft.Cuts[0].PlaybackRatePermille = 500
				draft.DurationMS = 60000
			case "overlap":
				draft.Cuts = append(draft.Cuts, clip.CorrectionCut{ID: ownerCutID, SourceID: draft.Cuts[0].SourceID, Fingerprint: draft.Cuts[0].Fingerprint, StartMS: 0, EndMS: 15000, Creation: &clip.CutCreation{Kind: clip.CutAdd, OriginID: draft.Cuts[0].ID}})
				draft.DurationMS = 45000
			case "authored-interval":
				if len(draft.Elements) == 0 {
					t.Fatal("fixture has no authored element")
				}
				start, end := 2000, 1000
				draft.Elements[0].StartMS, draft.Elements[0].EndMS = &start, &end
			}
			before, _ := json.Marshal(draft)
			observed, planned := h.planner.observe, h.planner.plans
			_, err = h.service.SaveCorrection(t.Context(), "alice", h.project.ID, p.EditPlanRevision, draft)
			if err == nil {
				t.Fatal("invalid draft was accepted")
			}
			var violation interface{ OutputValidationCode() string }
			if _, ok := clip.DiagnosticFromError(err); !ok && !errors.As(err, &violation) {
				t.Fatal("untyped rejection", err)
			}
			after, _ := json.Marshal(draft)
			if !bytes.Equal(before, after) {
				t.Fatal("validation silently rewrote the draft")
			}
			stored, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
			if err != nil || stored.EditPlan != p.EditPlan || stored.EditPlanRevision != p.EditPlanRevision || !reflect.DeepEqual(stored.Result, p.Result) || stored.RenderedPlanRevision != p.RenderedPlanRevision {
				t.Fatal("rejection changed the saved assembly or previous result", err)
			}
			if h.planner.observe != observed || h.planner.plans != planned {
				t.Fatal("validation spent an unapproved provider call")
			}
		})
	}
}
