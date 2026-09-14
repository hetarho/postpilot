package store_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func audioChange(h *generationHarness, p clip.Project, retain bool) clip.SourceAudioChange {
	s := h.batch.Sources[0]
	return clip.SourceAudioChange{ProjectID: h.project.ID, BatchID: h.batch.ID, SourceID: s.ID, Fingerprint: s.Fingerprint, RetainOriginal: retain, ExpectedRevision: p.EditPlanRevision}
}

// A freshly generated plan carries the owner's COMPLETE source-sound snapshot,
// stated by the server after the model's output was validated. The model never
// saw the setting, so changing it afterwards leaves the validated assembly
// reusable: continuation renders again without another AI call or charge
// (CLIP-100, CLIP-93).
func TestGenerationStatesTheOwnerSnapshotAndTheSettingDoesNotInvalidateThePlan(t *testing.T) {
	_, p, _ := completedClip(t)
	plan, _, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourceAudio == nil {
		t.Fatal("the generated plan carries no source-sound snapshot")
	}
	used := map[string]bool{}
	for _, c := range plan.Cuts {
		used[c.SourceID+"\x00"+c.Fingerprint] = true
	}
	if len(plan.SourceAudio.Values) != len(used) {
		t.Fatalf("the snapshot names %d sources, the cuts use %d", len(plan.SourceAudio.Values), len(used))
	}
	for _, v := range plan.SourceAudio.Values {
		if !used[v.SourceID+"\x00"+v.Fingerprint] || v.RetainOriginal {
			t.Fatalf("snapshot entry is foreign or defaulted to audible: %+v", v)
		}
	}
}

// An interrupted attempt keeps its validated candidate through a sound change:
// the setting was never part of the model's input or of the plan's semantic
// digest, so continuation resumes at rendering with no AI call and no charge.
func TestTheSoundSettingDoesNotInvalidateAnInterruptedCandidate(t *testing.T) {
	h := generationSetup(t)
	ctx := context.Background()
	h.renderer.fail = clip.ErrInvalidMedia
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected rendering failure")
	}
	before, err := h.service.Quote(ctx, "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil || !before.Pricing.SkipPlan || before.Pricing.ReusedChunks != 3 {
		t.Fatalf("the fixture has no reusable candidate: %+v %v", before.Pricing, err)
	}
	p, err := h.projects.GetProject(ctx, "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	observed, planned := h.planner.observe, h.planner.plans
	if _, _, err = h.sources.SetOriginalSound(ctx, "alice", audioChange(h, p, true)); err != nil {
		t.Fatal(err)
	}
	after, err := h.service.Quote(ctx, "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if !after.Pricing.SkipPlan || after.Pricing.ObservationCalls != 0 || after.Pricing.ReusedChunks != 3 || after.Pricing.MaxCredits != 0 {
		t.Fatalf("the sound setting invalidated observation or assembly work: %+v", after.Pricing)
	}
	if h.planner.observe != observed || h.planner.plans != planned {
		t.Fatal("the sound change consulted a model")
	}
	h.renderer.fail = nil
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal("render-only continuation failed", err)
	}
	if h.planner.observe != observed || h.planner.plans != planned || h.renderer.calls != 2 {
		t.Fatal("continuation repeated completed AI work")
	}
	if !h.renderer.plan.RetainsOriginalAudio(h.renderer.plan.Cuts[0]) {
		t.Fatal("continuation rendered the old source-sound snapshot")
	}
	completed, err := h.projects.GetProject(ctx, "alice", h.project.ID)
	if err != nil || completed.Result == nil {
		t.Fatal("continuation did not persist a result", err)
	}
	plan, _, err := clip.DecodeEditPlan(completed.EditPlan)
	if err != nil || !plan.RetainsOriginalAudio(plan.Cuts[0]) {
		t.Fatal("saved assembly lost the latest owner sound setting", err)
	}
	h.assertClean(t)
}

func TestSourceSoundChangeIsAtomicIdempotentAndRenderOnly(t *testing.T) {
	h, p, _ := completedClip(t)
	ctx := context.Background()
	before, _, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	if p.RenderedPlanRevision == 0 || p.Result == nil {
		t.Fatal("the fixture has no rendered result to keep")
	}
	observed, planned := h.planner.observe, h.planner.plans
	// Every new source starts off, whatever its per-cut volume says.
	if h.batch.Sources[0].RetainOriginalAudio || before.RetainsOriginalAudio(before.Cuts[0]) {
		t.Fatal("a new source defaulted to audible")
	}
	// A no-op changes nothing at all: not the revision, not the retention.
	batch, project, err := h.sources.SetOriginalSound(ctx, "alice", audioChange(h, p, false))
	if err != nil {
		t.Fatal(err)
	}
	if project.EditPlanRevision != p.EditPlanRevision || project.EditPlan != p.EditPlan || batch.Sources[0].RetainOriginalAudio {
		t.Fatal("an idempotent request moved the project", project.EditPlanRevision)
	}
	retention := batch.Sources[0].ExpiresAt

	batch, project, err = h.sources.SetOriginalSound(ctx, "alice", audioChange(h, p, true))
	if err != nil {
		t.Fatal(err)
	}
	if !batch.Sources[0].RetainOriginalAudio {
		t.Fatal("the lease did not record the owner's choice")
	}
	if project.EditPlanRevision != p.EditPlanRevision+1 {
		t.Fatal("the plan revision did not advance exactly once", project.EditPlanRevision)
	}
	// The existing result is stale, not deleted: a credit-free rerender can use it.
	if project.RenderedPlanRevision != p.RenderedPlanRevision || project.Result == nil {
		t.Fatal("the completed result was discarded", project.RenderedPlanRevision)
	}
	if !batch.Sources[0].ExpiresAt.After(retention) {
		t.Fatal("retention was not renewed")
	}
	after, _, err := clip.DecodeEditPlan(project.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	if !after.RetainsOriginalAudio(after.Cuts[0]) {
		t.Fatal("the plan snapshot disagrees with the lease")
	}
	// Nothing else moved: the same cuts, rates, per-cut volume and composition.
	expected := before
	expected.SourceAudio = after.SourceAudio
	if !reflect.DeepEqual(after, expected) {
		t.Fatal("changing source sound changed the assembly")
	}
	if project.Analysis != p.Analysis {
		t.Fatal("observation was invalidated")
	}
	if h.planner.observe != observed || h.planner.plans != planned {
		t.Fatal("an owner toggle spent a provider call", h.planner.observe, h.planner.plans)
	}
	// Per-cut volume is a gain, not a permission: turning the source off leaves
	// it exactly where it was.
	_, project, err = h.sources.SetOriginalSound(ctx, "alice", clip.SourceAudioChange{ProjectID: h.project.ID, BatchID: h.batch.ID, SourceID: h.batch.Sources[0].ID, Fingerprint: h.batch.Sources[0].Fingerprint, ExpectedRevision: project.EditPlanRevision})
	if err != nil {
		t.Fatal(err)
	}
	silent, _, err := clip.DecodeEditPlan(project.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	if silent.RetainsOriginalAudio(silent.Cuts[0]) || silent.Cuts[0].OriginalVolume() != before.Cuts[0].OriginalVolume() {
		t.Fatal("per-cut volume followed the source setting", silent.Cuts[0].OriginalVolume())
	}
}

func TestSourceSoundChangeRefusesEveryUnownedOrStaleRequest(t *testing.T) {
	h, p, _ := completedClip(t)
	ctx := context.Background()
	s := h.batch.Sources[0]
	base := audioChange(h, p, true)
	for name, c := range map[string]struct {
		user   string
		change clip.SourceAudioChange
		want   error
	}{
		"foreign owner":     {"bob", base, clip.ErrNotFound},
		"unknown project":   {"alice", replace(base, func(v *clip.SourceAudioChange) { v.ProjectID = "nope" }), clip.ErrNotFound},
		"unknown batch":     {"alice", replace(base, func(v *clip.SourceAudioChange) { v.BatchID = "nope" }), clip.ErrNotFound},
		"unknown source":    {"alice", replace(base, func(v *clip.SourceAudioChange) { v.SourceID = "nope" }), clip.ErrNotFound},
		"stale fingerprint": {"alice", replace(base, func(v *clip.SourceAudioChange) { v.Fingerprint = strings.Repeat("f", 64) }), clip.ErrNotFound},
		"stale revision":    {"alice", replace(base, func(v *clip.SourceAudioChange) { v.ExpectedRevision = p.EditPlanRevision + 1 }), clip.ErrPlanConflict},
		"pre-plan zero":     {"alice", replace(base, func(v *clip.SourceAudioChange) { v.ExpectedRevision = 0 }), clip.ErrPlanConflict},
		"empty source":      {"alice", replace(base, func(v *clip.SourceAudioChange) { v.SourceID = "" }), clip.ErrInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := h.sources.SetOriginalSound(ctx, c.user, c.change); !errors.Is(err, c.want) {
				t.Fatalf("got %v, wanted %v", err, c.want)
			}
		})
	}
	// A replacement batch takes over the project: the old one is no longer
	// editable even though its leases still exist.
	newer := rerenderBatch(t, h, false)
	if _, _, err := h.sources.SetOriginalSound(ctx, "alice", base); !errors.Is(err, clip.ErrSourceState) {
		t.Fatal("a replaced batch stayed editable", err)
	}
	current := clip.SourceAudioChange{ProjectID: h.project.ID, BatchID: newer.ID, SourceID: newer.Sources[0].ID, Fingerprint: newer.Sources[0].Fingerprint, RetainOriginal: true, ExpectedRevision: p.EditPlanRevision}
	if _, _, err := h.sources.SetOriginalSound(ctx, "alice", current); err != nil {
		t.Fatal("the current batch was refused", err)
	}
	_ = s
}

// A source the owner already decided about keeps that decision when the same
// file is reselected; a genuinely new file starts off.
func TestReselectedSourceInheritsTheOwnersResolvedChoice(t *testing.T) {
	h, p, _ := completedClip(t)
	ctx := context.Background()
	if _, _, err := h.sources.SetOriginalSound(ctx, "alice", audioChange(h, p, true)); err != nil {
		t.Fatal(err)
	}
	again := rerenderBatch(t, h, false)
	for _, v := range again.Sources {
		want := v.Fingerprint == h.batch.Sources[0].Fingerprint
		if v.RetainOriginalAudio != want {
			t.Fatalf("%s inherited %v, wanted %v", v.Fingerprint, v.RetainOriginalAudio, want)
		}
	}
}

func replace(v clip.SourceAudioChange, f func(*clip.SourceAudioChange)) clip.SourceAudioChange {
	f(&v)
	return v
}
