package design

import (
	"math"
	"slices"
	"strings"
)

const ViolationRegion Violation = "plan_layout_region"

// VerifyRegion is V20: it lays the region out again from all of its rows — the
// whole region by slot, since one slot's fit moves every other (CDS-87) — and
// requires this entry's parts to be exactly that layout's lines for the slots it
// draws, plus the preset's rules when it is the entry that paints them
// (CLIP-147). `offset` is the first slot the entry draws and `drawn` how many.
func VerifyRegion(kind, id, ratio string, regionRows []string, offset, drawn int, rules bool, parts Manifest) error {
	parts = slices.DeleteFunc(slices.Clone(parts), func(p Element) bool { return p.Kind == "scrim" })
	preset, ok := Region(kind, id)
	if !ok || offset < 0 || drawn < 0 || offset+drawn > len(preset.Slots()) {
		return ViolationRegion
	}
	for _, row := range regionRows {
		if strings.ContainsAny(row, "\r\n") {
			return ViolationRegion
		}
	}
	layout, err := LayoutRegion(kind, id, ratio, regionRows)
	if err != nil || layout.Over {
		return ViolationRegion
	}
	close := func(a, b float64) bool { return !math.IsNaN(a) && math.Abs(a-b) < .001 }
	next := 0
	for _, slot := range layout.Slots {
		if slot.Index < offset || slot.Index >= offset+drawn {
			continue
		}
		fill, alpha := slot.Spec.Paint()
		for k, line := range slot.Lines {
			if next >= len(parts) {
				return ViolationRegion
			}
			p := parts[next]
			centred := close(p.Region.X+p.Region.Width/2, layout.AnchorX)
			if layout.Align == "left" {
				centred = close(p.Region.X, layout.AnchorX)
			}
			if p.Kind != "copy" || p.Slot != slot.Index+1 || p.Line != k || p.Text != line.Text || !close(p.FontSize, line.Size) || !close(p.Floor, slot.Type.Floor) || p.FontSize < p.Floor || p.Fill != fill || !close(p.Opacity, alpha) || !close(p.BaselineY, line.Baseline) || !close(p.Region.Y, p.BaselineY+p.GlyphOffsetY) || !centred || p.Region.Width > layout.Width+1 {
				return ViolationRegion
			}
			next++
		}
	}
	if next == 0 {
		if len(parts) != 0 {
			return ViolationRegion
		}
		return nil
	}
	if rules {
		for _, rule := range layout.Rules {
			if next >= len(parts) {
				return ViolationRegion
			}
			p := parts[next]
			if p.Kind != "plate" || p.Rule != rule.Kind || p.Fill != Color["text_white"].Hex || !close(p.Opacity, rule.Alpha) || !close(p.Region.X, rule.Box.X) || !close(p.Region.Y, rule.Box.Y) || !close(p.Region.Width, rule.Box.Width) || !close(p.Region.Height, rule.Box.Height) {
				return ViolationRegion
			}
			next++
		}
	}
	if next != len(parts) {
		return ViolationRegion
	}
	return nil
}
