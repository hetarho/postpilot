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
