package clip_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func nativeHistoryFixture(t *testing.T) (clip.Project, clip.EditPlan) {
	t.Helper()
	p, _ := correctionFixture(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Cuts[0].EndMS, plan.Cuts[1].EndMS, plan.DurationMS = 20000, 20000, 39800
	plan.Portable, err = clip.FreezeLegacyPlan(p, plan, clip.Recipe{}, clip.DefaultCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	plan.Portable.NativeEditing = true
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	return p, plan
}

func TestNativeUndoRestoresSavedDeletedIdentityAndKeepsOutputRelativeText(t *testing.T) {
	p, plan := nativeHistoryFixture(t)
	global := plan.Portable.Elements[0]
	global.Resolved.InstanceID, global.Resolved.Element.ID, global.Resolved.Element.Basis = "global", "global", "whole"
	global.Resolved.Element.StartMS, global.Resolved.Element.EndMS = nil, nil
	global.Resolved.StartMS, global.Resolved.EndMS = 0, plan.DurationMS
	plan.Portable.Elements = append(plan.Portable.Elements, global)
	p.EditPlan, _ = clip.EncodeEditPlan(plan)
	original := clip.CorrectionFromPlan(plan)
	removed := original
	removed.Cuts = slices.Clone(original.Cuts[1:])
	removed.Cuts[0].TransitionMS = 0
	removed.DurationMS = 20000
	removed.Elements = nil
	for _, e := range original.Elements {
		if e.CutID != "first" || e.Basis != "cut" {
			removed.Elements = append(removed.Elements, e)
		}
	}
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, removed)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Portable.RetiredCuts) != 1 || len(next.Portable.RetiredElements) != 1 || len(next.Portable.Elements) != 2 {
		t.Fatal("lost undo identity or global text")
	}
	p.EditPlan, err = clip.EncodeEditPlan(next)
	if err != nil {
		t.Fatal("deleted plan", err)
	}
	restored, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, original)
	if err != nil {
		t.Fatal("undo after save", err)
	}
	if len(restored.Cuts) != 2 || len(restored.Portable.Elements) != 3 || restored.Portable.Elements[0].Resolved.Text != plan.Portable.Elements[0].Resolved.Text {
		t.Fatal("undo lost content")
	}
	original.Cuts[0].ID = "forged"
	if _, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, original); err == nil {
		t.Fatal("accepted forged history identity")
	}
}

func TestAssociationCorrectionPreservesObservationsAndFactsUntilOwnerReviews(t *testing.T) {
	p, plan := nativeHistoryFixture(t)
	plan.Portable.Snapshot.Body = `<clip version="1" intro="b" caption="bold" outro="e"><group id="menu"><field id="price" label="Price"/></group><repeat for="scenes"><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the subject</text></scene></repeat><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
	plan.Portable.Inputs.Items = map[string][]composition.Item{"menu": {{ID: "rice", Values: map[string]string{"price": "8000"}}, {ID: "soup", Values: map[string]string{"price": "12000"}}}}
	observed, err := clip.RetainedObservations(p)
	if err != nil {
		t.Fatal(err)
	}
	observed[0].Segments = []clip.Segment{{StartMS: 0, EndMS: 20000}}
	raw, _ := json.Marshal(observed)
	p.Analysis = string(raw)
	plan.Portable.Elements[0].Resolved.Element.Kind = "ai"
	plan.Portable.Elements[0].Resolved.Text = "rice 8000"
	plan.Portable.Elements[0].Resolved.GroupID, plan.Portable.Elements[0].Resolved.ItemID = "menu", "rice"
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	draft := clip.CorrectionFromPlan(plan)
	associations := []clip.SourceAssociation{{GroupID: "menu", ItemID: "soup", SourceID: "a", Fingerprint: "fa", StartMS: 0, EndMS: 20000}}
	draft.Associations = &associations
	changed, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.Portable.Elements[0].StaleEvidence || changed.Portable.Elements[0].Resolved.Text != "rice 8000" || !reflect.DeepEqual(changed.Portable.Inputs.Items, plan.Portable.Inputs.Items) || !reflect.DeepEqual(changed.Portable.Elements[0].Evidence, plan.Portable.Elements[0].Evidence) {
		t.Fatal("rewrote item facts or lost stale claim")
	}
	if clip.ValidateCompositionEvidence(changed) == nil {
		t.Fatal("stale claim can render")
	}
	p.EditPlan, err = clip.EncodeEditPlan(changed)
	if err != nil {
		t.Fatal(err)
	}
	review := clip.CorrectionFromPlan(changed)
	review.Elements[0].EvidenceReviewed = true
	corrected, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, review)
	if err != nil || clip.ValidateCompositionEvidence(corrected) != nil {
		t.Fatal("explicit review", err)
	}
	if p.Analysis != string(raw) {
		t.Fatal("changed raw observations")
	}
}

func TestNativeCorrectionDoesNotEnableTheLegacyEditingMarker(t *testing.T) {
	p, plan := nativeHistoryFixture(t)
	plan.Portable.Snapshot.Legacy, plan.Portable.NativeEditing = false, false
	p.EditPlan, _ = clip.EncodeEditPlan(plan)
	draft := clip.CorrectionFromPlan(plan)
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if next.Portable.NativeEditing {
		t.Fatal("native correction acquired an unnecessary legacy marker")
	}
	p.EditPlan, _ = clip.EncodeEditPlan(next)
	again, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, clip.CorrectionFromPlan(next))
	if err != nil || !reflect.DeepEqual(next, again) {
		t.Fatal("unchanged correction is not idempotent", err)
	}
}
