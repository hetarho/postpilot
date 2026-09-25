package design

import (
	"math"
	"slices"
	"strings"
)

const ViolationRegion Violation = "plan_layout_region"

// VerifyRegion is V20: it lays the region out again from all of its rows — the
// whole region by slot, since one slot's fit moves every other (CDS-87) — and
// requires this entry's parts to be exactly that layout's lines and slot-bound
// decoration for the slots it draws, plus the preset's rules and block
// decoration when it is the entry that paints them (CLIP-147, CDS-73). A turned
// part is checked on the unrotated layout and must carry the block's turn.
// `offset` is the first slot the entry draws and `drawn` how many.
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
	same := func(a, b Bounds) bool {
		return close(a.X, b.X) && close(a.Y, b.Y) && close(a.Width, b.Width) && close(a.Height, b.Height)
	}
	turned := func(p Element, rotated bool) bool {
		if rotated {
			return close(p.Rotate, layout.Rotate.Deg)
		}
		return p.Rotate == 0
	}
	next := 0
	shape := func(s RegionShape) bool {
		if next >= len(parts) {
			return false
		}
		p := parts[next]
		fill, alpha := s.Fill, s.FillAlpha
		if fill == "" {
			fill, alpha = s.Stroke, s.StrokeAlpha
		}
		if p.Kind != "plate" || p.Rule != s.Kind || p.Fill != fill || !close(p.Opacity, alpha) || !same(p.Region, s.Box) || !turned(p, s.Rotated) {
			return false
		}
		next++
		return true
	}
	lines := 0
	for _, slot := range layout.Slots {
		if slot.Index < offset || slot.Index >= offset+drawn {
			continue
		}
		fill, alpha := slot.Spec.Paint()
		width := preset.SlotWidth(ratio, slot.Index)
		for k, line := range slot.Lines {
			if next >= len(parts) {
				return ViolationRegion
			}
			p := parts[next]
			var placed bool
			switch {
			case line.Arc != nil:
				placed = same(p.Region, slot.Box)
			case line.Align == "left":
				placed = close(p.Region.X, line.X) && p.Region.Width <= width+1
			default:
				placed = close(p.Region.X+p.Region.Width/2, line.X) && p.Region.Width <= width+1
			}
			if p.Kind != "copy" || p.Slot != slot.Index+1 || p.Line != k || p.Text != line.Text || !close(p.FontSize, line.Size) || !close(p.Floor, slot.Type.Floor) || p.FontSize < p.Floor || p.Fill != fill || !close(p.Opacity, alpha) || !close(p.BaselineY, line.Baseline) || !close(p.Region.Y, p.BaselineY+p.GlyphOffsetY) || !placed || !turned(p, line.Rotated) {
				return ViolationRegion
			}
			next++
			lines++
		}
		for _, s := range layout.Shapes {
			if s.Slot == slot.Index && !shape(s) {
				return ViolationRegion
			}
		}
	}
	if lines == 0 {
		if len(parts) != 0 {
			return ViolationRegion
		}
		return nil
	}
	if rules {
		white := Color["text_white"].Hex
		for _, rule := range layout.Rules {
			if next >= len(parts) {
				return ViolationRegion
			}
			p := parts[next]
			if p.Kind != "plate" || p.Rule != rule.Kind || p.Fill != white || !close(p.Opacity, rule.Alpha) || !same(p.Region, rule.Box) || !turned(p, layout.Rotate.Deg != 0) {
				return ViolationRegion
			}
			next++
		}
		for _, s := range layout.Shapes {
			if s.Slot < 0 && !shape(s) {
				return ViolationRegion
			}
		}
	}
	if next != len(parts) {
		return ViolationRegion
	}
	return nil
}
