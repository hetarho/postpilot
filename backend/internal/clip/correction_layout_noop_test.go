package clip

import "testing"

func TestCorrectionNoopIgnoresReadOnlyAutomaticAnchor(t *testing.T) {
	stored := PortableText{Placement: &CompositionPlacement{Style: "bold", Position: "lower_mid", StartMS: 120, EndMS: 3000}}
	stored.Resolved.InstanceID = "caption"
	stored.Resolved.Text = "현재 장면"
	stored.Resolved.Element.Position = "auto"
	before := correctionText(stored)
	wire := before
	wire.EffectivePosition = "" // The RPC deliberately ignores client-authored read fields.
	unchanged := applyTextEdit(stored, wire, nil)
	if unchanged.Placement == nil || unchanged.Placement.Position != "lower_mid" || unchanged.OwnerEdited {
		t.Fatalf("wire no-op discarded automatic geometry: %+v", unchanged)
	}
	wire.Text = "수정한 장면"
	changed := applyTextEdit(stored, wire, nil)
	if changed.Placement != nil || !changed.OwnerEdited || changed.Resolved.Text != wire.Text {
		t.Fatalf("real edit retained old automatic geometry: %+v", changed)
	}
}
