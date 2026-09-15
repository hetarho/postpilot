package design

import (
	"math"
	"slices"
	"strings"
)

const ViolationRegion Violation = "plan_layout_region"

func RegionType(slot RegionSlot, ratio string) TypeRole {
	role := Type[slot.Type]
	if slot.Type == "hook" {
		layout, _ := Layout(ratio)
		role.Size = layout.HookSize
	}
	if slot.Tracking != nil {
		role.Tracking = *slot.Tracking
	}
	return role
}

func regionScale(ratio string) float64 {
	layout, _ := Layout(ratio)
	base, _ := Layout("vertical")
	return float64(layout.Canvas.Height) / float64(base.Canvas.Height)
}

// RegionOffset is CDS-79: a block keeps its 9:16 slot spacing and rule offsets
// on every ratio and moves as one piece, its centre landing on the same
// fraction of canvas height it holds on 9:16. The centre is the midpoint of the
// preset's own y values — slot baselines and rule offsets together, never glyph
// metrics — so the offset stays a pure function of the preset and the ratio,
// and is 0 on 9:16 by construction.
func RegionOffset(preset RegionPreset, ratio string) float64 {
	low, high := math.Inf(1), math.Inf(-1)
	for _, slot := range preset.Slots {
		low, high = math.Min(low, slot.Y), math.Max(high, slot.Y)
	}
	for _, rule := range preset.Rules {
		low, high = math.Min(low, rule.Y), math.Max(high, rule.Y)
	}
	if math.IsInf(low, 1) {
		return 0
	}
	centre := (low + high) / 2
	return centre*regionScale(ratio) - centre
}

// RegionBaseline places one of the preset's own y values — a slot baseline or a
// rule offset — on the ratio's canvas. The renderer and V20 both resolve every
// region y through it, so neither can drift from the other.
func RegionBaseline(preset RegionPreset, ratio string, y float64) float64 {
	return y + RegionOffset(preset, ratio)
}

// VerifyRegion is V20: omitted slots retain their indices and every painted
// slot/rule must match the selected preset. Glyph offsets are recorded during
// shaping so a moved text box cannot keep an unchanged declared baseline.
func VerifyRegion(kind, id, ratio string, rows []string, parts Manifest) error {
	parts = slices.DeleteFunc(slices.Clone(parts), func(p Element) bool { return p.Kind == "scrim" })
	preset, ok := Region(kind, id)
	if !ok || len(rows) > len(preset.Slots) {
		return ViolationRegion
	}
	layout, ok := Layout(ratio)
	if !ok {
		return ViolationRegion
	}
	close := func(a, b float64) bool { return !math.IsNaN(a) && math.Abs(a-b) < .001 }
	next := 0
	for i, text := range rows {
		if strings.TrimSpace(text) == "" {
			continue
		}
		if next >= len(parts) {
			return ViolationRegion
		}
		p, slot := parts[next], preset.Slots[i]
		role := RegionType(slot, ratio)
		alpha := Color[slot.Fill].Alpha
		if slot.Alpha != nil {
			alpha *= *slot.Alpha
		}
		if p.Kind != "copy" || p.Slot != i+1 || p.Text != text || strings.ContainsAny(text, "\r\n") || Chars(text) > role.Chars || !close(p.FontSize, role.Size) || p.Fill != Color[slot.Fill].Hex || !close(p.Opacity, alpha) || !close(p.BaselineY, RegionBaseline(preset, ratio, slot.Y)) || !close(p.Region.Y, p.BaselineY+p.GlyphOffsetY) || !close(p.Region.X+p.Region.Width/2, layout.Anchor.Center) {
			return ViolationRegion
		}
		next++
	}
	if next == 0 {
		if len(parts) != 0 {
			return ViolationRegion
		}
		return nil
	}
	for _, line := range preset.Rules {
		if next >= len(parts) {
			return ViolationRegion
		}
		p, rule := parts[next], Rules[line.Kind]
		if p.Kind != "plate" || p.Rule != line.Kind || p.Fill != Color["text_white"].Hex || !close(p.Opacity, rule.Alpha) || !close(p.Region.Y, RegionBaseline(preset, ratio, line.Y)) || !close(p.Region.X, layout.Anchor.Center-rule.Width/2) || !close(p.Region.Width, rule.Width) || !close(p.Region.Height, rule.Height) {
			return ViolationRegion
		}
		next++
	}
	if next != len(parts) {
		return ViolationRegion
	}
	return nil
}
