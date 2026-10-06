package clip_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestStoredAutomaticAnchorProjectsWithoutChangingItsAuthoredPosition(t *testing.T) {
	p, plan := nativeHistoryFixture(t)
	text := &plan.Portable.Elements[0]
	text.Resolved.Element.Position = "auto"
	text.Placement = &clip.CompositionPlacement{Position: "lower_mid", Style: "bold", StartMS: 120, EndMS: 3000}
	before := clip.CorrectionFromPlan(plan).Elements[0]
	if before.Position != "auto" || before.EffectivePosition != "lower_mid" {
		t.Fatal("lost the stored chosen anchor or changed authored placement", before)
	}
	var err error
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	identity := clip.BrowserCompositionIdentity(p, plan, nil)
	text.Placement.Position = "upper_mid"
	p.EditPlan, err = clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if clip.BrowserCompositionIdentity(p, plan, nil) == identity {
		t.Fatal("authoritative browser identity did not bind the persisted chosen anchor")
	}
}
