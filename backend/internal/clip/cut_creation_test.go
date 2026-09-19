package clip_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

const ownerCut = "owner-3f2504e0-4f89-41d3-9a0c-0305e82c3301"
const otherOwnerCut = "owner-6ba7b810-9dad-41d1-80b4-00c04fd430c8"

// creationFixture is a native project whose two cuts come from one observed
// source: the shape every add, split and undo case below starts from.
func creationFixture(t *testing.T) (clip.Project, clip.EditPlan) {
	t.Helper()
	info := cadence30(60000)
	segments := []clip.Segment{
		{StartMS: 0, EndMS: 30000, Event: "scene", Quality: "usable", Focal: clip.Point{X: .25, Y: .75}},
		{StartMS: 30000, EndMS: 60000, Event: "scene", Quality: "usable", Focal: clip.Point{X: .5, Y: .5}},
	}
	sources := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "a", Fingerprint: "fa", Info: info}, Filename: "a.mp4"}, Segments: segments}}
	analysis, _ := json.Marshal(sources)
	copies := []clip.Caption{{Text: "조용한 골목", Anchor: "bottom", Align: "center", Style: "clean"}}
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 20000, Cuts: []clip.Cut{
		{ID: "first", SourceID: "a", Fingerprint: "fa", StartMS: 0, EndMS: 10000, Focal: clip.Point{X: .3, Y: .4}, Copies: copies},
		{ID: "second", SourceID: "a", Fingerprint: "fa", StartMS: 10000, EndMS: 20000, TransitionMS: 0, Focal: clip.Point{X: .5, Y: .5}, Copies: copies},
	}}
	p := clip.Project{Ratio: "vertical", Analysis: string(analysis), EditPlanRevision: 1}
	portable, err := clip.FreezeLegacyPlan(p, plan, clip.Recipe{Accent: "teal"}, clip.DefaultCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	plan.Portable = portable
	plan.Portable.NativeEditing = true
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	p.EditPlan = raw
	saved, err := clip.DecodeEditPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	saved.Portable.NativeEditing = true
	return p, saved
}

func creationDraft(plan clip.EditPlan) clip.CorrectionPlan {
	draft := clip.CorrectionFromPlan(plan)
	draft.NativeComposition = true
	return draft
}

func addedCut(id, origin string, start, end int) clip.CorrectionCut {
	return clip.CorrectionCut{ID: id, SourceID: "a", Fingerprint: "fa", StartMS: start, EndMS: end, VolumePermille: 200,
		Focal: &clip.Point{X: 0, Y: 0}, Creation: &clip.CutCreation{Kind: clip.CutAdd, OriginID: origin}}
}

func TestOwnerAddCreatesFootageWithServerOwnedDefaults(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	draft := creationDraft(plan)
	// A second look at the same observed scene the first cut came from.
	draft.Cuts = append(draft.Cuts, addedCut(ownerCut, "first", 20000, 30000))
	draft.DurationMS = 30000
	next, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	created := next.Cuts[2]
	if created.ID != ownerCut || created.StartMS != 20000 || created.EndMS != 30000 {
		t.Fatal("the submitted range was not the one stored", created)
	}
	// Nothing the client sent about focal, volume, rate or transition survived.
	if created.Rate() != clip.RateUnitPermille || created.TransitionMS != 0 || created.OriginalVolume() != 1 {
		t.Fatal("client-owned defaults leaked into a created cut", created)
	}
	if created.Focal != (clip.Point{X: .25, Y: .75}) {
		t.Fatal("the focal point was not the observation's", created.Focal)
	}
	if len(created.Copies) != 0 || len(created.Chips) != 0 {
		t.Fatal("a created cut carried text", created)
	}
	for _, e := range next.Portable.Elements {
		if e.Resolved.CutID == ownerCut {
			t.Fatal("a caption was copied onto the new cut")
		}
	}
	// It inherits its place in the template from the origin, and is stored
	// without any trace of the provenance that authorized it.
	binding, ok := portableBinding(next, ownerCut)
	if !ok || binding.SectionID != "footage" {
		t.Fatal("the new cut did not inherit the template section", binding)
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "Creation") || strings.Contains(raw, "OriginID") {
		t.Fatal("creation metadata reached the stored plan")
	}
	// The projection hands it back as an ordinary cut, so a resave is a
	// correction rather than a second creation.
	for _, c := range clip.CorrectionFromPlan(next).Cuts {
		if c.Creation != nil {
			t.Fatal("the projection claimed a cut was still being created", c.ID)
		}
	}
}

func TestOwnerSplitDividesFootageAndLeavesTextOnTheLeft(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	draft := creationDraft(plan)
	// The parent keeps its id and its content; the right side is new footage.
	draft.Cuts[0].EndMS = 4000
	right := clip.CorrectionCut{ID: ownerCut, StartMS: 4000, EndMS: 10000, VolumePermille: 0,
		Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: "first"}}
	draft.Cuts = slices.Insert(draft.Cuts, 1, right)
	next, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	left, created := next.Cuts[0], next.Cuts[1]
	if left.ID != "first" || left.StartMS != 0 || left.EndMS != 4000 {
		t.Fatal("the parent did not keep its identity and left range", left)
	}
	if created.SourceID != left.SourceID || created.Fingerprint != left.Fingerprint || created.Focal != plan.Cuts[0].Focal ||
		created.Rate() != plan.Cuts[0].Rate() || created.OriginalVolume() != plan.Cuts[0].OriginalVolume() {
		t.Fatal("the right cut did not inherit the parent's properties", created)
	}
	if created.TransitionMS != 0 {
		t.Fatal("the right cut did not start with a hard cut", created.TransitionMS)
	}
	// The authored caption stays exactly where it was authored: on the left.
	for _, e := range next.Portable.Elements {
		if e.Resolved.CutID == ownerCut {
			t.Fatal("text was duplicated onto the right cut")
		}
	}
	if binding, ok := portableBinding(next, ownerCut); !ok || binding.SectionID != "footage" {
		t.Fatal("the split lost its portable binding", binding)
	}
	// Source ranges stay adjacent and half-open, so a split is never an overlap.
	if len(clip.SourceOverlaps(next.Cuts)) != 0 {
		t.Fatal("a split produced overlapping footage")
	}
}

func TestOwnerCutCreationRefusesForgedIdentityAndUnevidencedFootage(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	// The reason each refusal must give, so a case cannot pass because some
	// unrelated check happened to fail first.
	reasons := map[string]string{
		"non-canonical id":                 "cut_identity",
		"creation on a known id":           "cut_identity",
		"no provenance":                    "cut_identity",
		"unknown origin":                   "cut_origin",
		"unknown kind":                     "cut_creation_kind",
		"foreign source":                   "cut_observation",
		"outside every observed scene":     "cut_observation",
		"split at the parent's start":      "cut_split_point",
		"split at the parent's end":        "cut_split_point",
		"split that extends the parent":    "cut_split_point",
		"text authored onto a created cut": "cut_created_content",
	}
	cases := map[string]func(*clip.CorrectionPlan){
		"non-canonical id": func(d *clip.CorrectionPlan) {
			d.Cuts = append(d.Cuts, addedCut("third", "first", 20000, 30000))
			d.DurationMS = 30000
		},
		"creation on a known id": func(d *clip.CorrectionPlan) {
			d.Cuts[0].Creation = &clip.CutCreation{Kind: clip.CutAdd, OriginID: "second"}
		},
		"no provenance": func(d *clip.CorrectionPlan) {
			c := addedCut(ownerCut, "first", 20000, 30000)
			c.Creation = nil
			d.Cuts = append(d.Cuts, c)
			d.DurationMS = 30000
		},
		"unknown origin": func(d *clip.CorrectionPlan) {
			d.Cuts = append(d.Cuts, addedCut(ownerCut, otherOwnerCut, 20000, 30000))
			d.DurationMS = 30000
		},
		"unknown kind": func(d *clip.CorrectionPlan) {
			c := addedCut(ownerCut, "first", 20000, 30000)
			c.Creation.Kind = "trim"
			d.Cuts = append(d.Cuts, c)
			d.DurationMS = 30000
		},
		"foreign source": func(d *clip.CorrectionPlan) {
			c := addedCut(ownerCut, "first", 20000, 30000)
			c.SourceID, c.Fingerprint = "b", "fb"
			d.Cuts = append(d.Cuts, c)
			d.DurationMS = 30000
		},
		"outside every observed scene": func(d *clip.CorrectionPlan) {
			// 25000..35000 straddles the boundary between the two scenes.
			d.Cuts = append(d.Cuts, addedCut(ownerCut, "first", 25000, 35000))
			d.DurationMS = 30000
		},
		"overlapping the plan's own footage": func(d *clip.CorrectionPlan) {
			d.Cuts = append(d.Cuts, addedCut(ownerCut, "first", 5000, 15000))
			d.DurationMS = 30000
		},
		"duplicate created id": func(d *clip.CorrectionPlan) {
			d.Cuts = append(d.Cuts, addedCut(ownerCut, "first", 20000, 30000), addedCut(ownerCut, "first", 30000, 40000))
			d.DurationMS = 40000
		},
		"split at the parent's start": func(d *clip.CorrectionPlan) {
			d.Cuts = slices.Insert(d.Cuts, 1, clip.CorrectionCut{ID: ownerCut, StartMS: 0, EndMS: 10000, Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: "first"}})
		},
		"split at the parent's end": func(d *clip.CorrectionPlan) {
			d.Cuts = slices.Insert(d.Cuts, 1, clip.CorrectionCut{ID: ownerCut, StartMS: 10000, EndMS: 10000, Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: "first"}})
		},
		"split that extends the parent": func(d *clip.CorrectionPlan) {
			d.Cuts[0].EndMS = 4000
			d.Cuts = slices.Insert(d.Cuts, 1, clip.CorrectionCut{ID: ownerCut, StartMS: 4000, EndMS: 14000, Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: "first"}})
			d.DurationMS = 24000
		},
		"split too short to survive the next fade": func(d *clip.CorrectionPlan) {
			d.Cuts[1].TransitionMS = 200
			d.DurationMS -= 200
			d.Cuts[0].EndMS = 9900
			d.Cuts = slices.Insert(d.Cuts, 1, clip.CorrectionCut{ID: ownerCut, StartMS: 9900, EndMS: 10000, Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: "first"}})
		},
		"text authored onto a created cut": func(d *clip.CorrectionPlan) {
			d.Cuts = append(d.Cuts, addedCut(ownerCut, "first", 20000, 30000))
			d.DurationMS = 30000
			copied := d.Elements[0]
			copied.InstanceID, copied.CutID = "copied", ownerCut
			d.Elements = append(d.Elements, copied)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			draft := creationDraft(plan)
			mutate(&draft)
			_, err := clip.ApplyCorrection(cfg, p, draft)
			if err == nil {
				t.Fatal("an invented cut was accepted")
			}
			if !errors.Is(err, clip.ErrInvalid) {
				t.Fatal("a refusal lost the invalid-plan identity", err)
			}
			want, named := reasons[name]
			var refusal *clip.CutError
			if !named {
				return
			}
			if !errors.As(err, &refusal) || refusal.OutputValidationCode() != want || refusal.CutID == "" {
				t.Fatalf("got %v, wanted %s on a named cut", err, want)
			}
		})
	}
}

func TestCreatedCutJoinsIdentityHistoryAndSurvivesUndo(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	draft := creationDraft(plan)
	draft.Cuts = append(draft.Cuts, addedCut(ownerCut, "first", 20000, 30000))
	draft.DurationMS = 30000
	added, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	p.EditPlan, err = clip.EncodeEditPlan(added)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	saved.Portable.NativeEditing = true
	// Delete it: the identity and its binding are retired, not forgotten.
	deleted := creationDraft(saved)
	deleted.Cuts = deleted.Cuts[:2]
	deleted.DurationMS = 20000
	gone, err := clip.ApplyCorrection(cfg, p, deleted)
	if err != nil {
		t.Fatal(err)
	}
	if len(gone.Portable.RetiredCuts) != 1 || gone.Portable.RetiredCuts[0].ID != ownerCut {
		t.Fatal("the created identity was not retired", gone.Portable.RetiredCuts)
	}
	p.EditPlan, err = clip.EncodeEditPlan(gone)
	if err != nil {
		t.Fatal(err)
	}
	// Undo restores exactly the validated cut, with no creation metadata.
	restored, err := clip.ApplyCorrection(cfg, p, creationDraft(saved))
	if err != nil {
		t.Fatal("undo after save", err)
	}
	if len(restored.Cuts) != 3 || restored.Cuts[2].ID != ownerCut || restored.Cuts[2].Focal != (clip.Point{X: .25, Y: .75}) {
		t.Fatal("undo did not restore the same cut", restored.Cuts)
	}
	if binding, ok := portableBinding(restored, ownerCut); !ok || binding.SectionID != "footage" {
		t.Fatal("undo lost the binding", binding)
	}
	// A retired identity cannot be resurrected by claiming to create it again.
	forged := creationDraft(saved)
	forged.Cuts[2].Creation = &clip.CutCreation{Kind: clip.CutAdd, OriginID: "first"}
	if _, err := clip.ApplyCorrection(cfg, p, forged); err == nil {
		t.Fatal("a retired identity was re-created")
	}
}

// A split leaves an authored window that no longer fits in the owner's draft
// rather than moving, shortening or dropping it (CLIP-67, CDS-64).
func TestSplitRefusesRatherThanRetimingAuthoredText(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	draft := creationDraft(plan)
	authored := 0
	for i, e := range draft.Elements {
		if e.CutID == "first" && e.Basis == "cut" {
			start, end := 200, 9000
			draft.Elements[i].StartMS, draft.Elements[i].EndMS = &start, &end
			authored++
		}
	}
	if authored == 0 {
		t.Fatal("the fixture has no cut-bound text to author")
	}
	before := *draft.Elements[0].EndMS
	draft.Cuts[0].EndMS = 4000
	draft.Cuts = slices.Insert(draft.Cuts, 1, clip.CorrectionCut{ID: ownerCut, StartMS: 4000, EndMS: 10000,
		Creation: &clip.CutCreation{Kind: clip.CutSplit, OriginID: "first"}})
	if _, err := clip.ApplyCorrection(cfg, p, draft); err == nil {
		t.Fatal("an out-of-range authored window was silently retimed")
	}
	if *draft.Elements[0].EndMS != before {
		t.Fatal("the owner's draft was rewritten", *draft.Elements[0].EndMS)
	}
}

func portableBinding(p clip.EditPlan, id string) (compositionCut, bool) {
	for _, c := range p.Portable.Cuts {
		if c.ID == id {
			return compositionCut{c.ID, c.SectionID, c.SourceID, c.GroupID, c.ItemID}, true
		}
	}
	return compositionCut{}, false
}

type compositionCut struct{ ID, SectionID, SourceID, GroupID, ItemID string }
