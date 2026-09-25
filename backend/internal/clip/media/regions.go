package media

import (
	"context"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

func trimmed(alpha float64) string { return strconv.FormatFloat(alpha, 'f', -1, 64) }

func inside(r, safe clip.Region) bool {
	return r.X >= safe.X && r.Y >= safe.Y && r.X+r.Width <= safe.X+safe.Width && r.Y+r.Height <= safe.Y+safe.Height
}

func regionBounds(visual declaredVisual) clip.Region {
	var bounds clip.Region
	for _, part := range visual.manifest.Parts {
		if part.Kind != "copy" {
			continue
		}
		if bounds.Width == 0 {
			bounds = clip.Region(part.Region)
		} else {
			bounds = unionRegion(bounds, clip.Region(part.Region))
		}
	}
	return bounds
}

// regionElements is the region's resolved entries with this entry's own
// current lines in place, so a retried alternative lays out as itself.
func regionElements(elements []composition.ResolvedElement, current composition.ResolvedElement) []composition.ResolvedElement {
	out := make([]composition.ResolvedElement, len(elements))
	for i, e := range elements {
		if e.InstanceID == current.InstanceID {
			e = current
		}
		out[i] = e
	}
	return out
}

// layoutDeclaredRegion draws this entry's slots of one region block. The block
// is laid out once from every line of the region (CDS-86, CDS-87), so a slot
// that shrinks or wraps moves its neighbours whichever entry supplies them; the
// entry draws only its own slots, and the one that paints the rules paints them
// at the block's positions (CLIP-147).
func (r *Rendering) layoutDeclaredRegion(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual, kind, id string, placement clip.RegionPlacement, regionRows []string) (declaredVisual, error) {
	if _, ok := design.Region(kind, id); !ok {
		return visual, elementProblem(visual.text, "invalid_design")
	}
	rows := visual.text.Resolved.Rows
	if len(rows) == 0 {
		rows = []composition.ResolvedRow{{Text: visual.text.Resolved.Text}}
	}
	// The lines this entry's own slots can hold; the rest are drawn by nobody
	// and noticed instead of refused (CLIP-147).
	rows = rows[:min(len(rows), max(0, placement.Drawn))]
	for _, row := range rows {
		if strings.ContainsAny(row.Text, "\r\n") {
			return visual, elementProblem(visual.text, "copy_limit")
		}
	}
	block, err := design.LayoutRegion(kind, id, ratio, regionRows)
	if err != nil {
		return visual, elementProblem(visual.text, "invalid_design")
	}
	shadow := overlayShadow("text")
	visual.region = overlay.RegionView{CopyView: overlay.CopyView{Canvas: overlay.Canvas{Width: canvas.Width, Height: canvas.Height}, Shadow: &shadow}}
	for i, row := range rows {
		index := placement.Offset + i
		slot, ok := block.Slot(index)
		if !ok || strings.TrimSpace(row.Text) == "" {
			continue
		}
		role := slot.Type
		if slot.Over {
			// Wider than its width at its floor, or a character the preset's
			// face does not draw (CDS-77).
			if !design.Covers(role.Face, role.Weight, row.Text) {
				return visual, elementProblem(visual.text, "unsupported_glyph")
			}
			return visual, elementProblem(visual.text, "copy_limit")
		}
		fill, alpha := slot.Spec.Paint()
		for k, line := range slot.Lines {
			bounds, err := r.lineBounds(ctx, ws, line.Text, role)
			if err != nil {
				return visual, declaredRoleError(visual.text, err)
			}
			x := block.AnchorX - bounds.Width/2
			if block.Align == "left" {
				x = block.AnchorX
			}
			box := clip.Region{X: x, Y: line.Baseline + bounds.Y, Width: bounds.Width, Height: bounds.Height}
			if !inside(box, canvas.Safe) {
				return visual, elementProblem(visual.text, "safe_area")
			}
			text := overlayText(role, line.Text, box.X-bounds.X, line.Baseline, fill, trimmed(alpha))
			text.Stroke, text.StrokeOpacity = "none", "1"
			if slot.Spec.Stroke == "text" || slot.Spec.Stroke == "small" {
				text.Stroke, text.StrokeOpacity = paint("stroke_dark")
				text.StrokeWidth = design.Spacing.StrokeText
				if slot.Spec.Stroke == "small" {
					text.StrokeWidth = design.Spacing.StrokeSmall
				}
			}
			text.Shadow = slot.Spec.Shadow != ""
			visual.region.Lines = append(visual.region.Lines, text)
			visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "copy", Slot: index + 1, Line: k, Text: line.Text, FontSize: role.Size, Floor: role.Floor, Fill: fill, Opacity: alpha, BaselineY: line.Baseline, GlyphOffsetY: bounds.Y, Region: design.Bounds(box), StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
		}
	}
	// The preset's own rules are painted once for the region, by the first entry
	// that has a line to paint (CDS-73, CLIP-147).
	if placement.Rules && len(visual.region.Lines) > 0 {
		white := design.Color["text_white"].Hex
		for _, rule := range block.Rules {
			box := clip.Region(rule.Box)
			visual.region.Rules = append(visual.region.Rules, overlayBox(box, 0, white, trimmed(rule.Alpha)))
			visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "plate", Rule: rule.Kind, Fill: white, Opacity: rule.Alpha, Region: design.Bounds(box), StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
		}
	}
	visual.manifest.Region = regionBounds(visual)
	for _, part := range visual.manifest.Parts {
		visual.manifest.Region = unionRegion(visual.manifest.Region, clip.Region(part.Region))
	}
	if err := design.VerifyRegion(kind, id, ratio, regionRows, placement.Offset, placement.Drawn, placement.Rules, visual.manifest.Parts); err != nil {
		return visual, elementProblem(visual.text, "preset_mismatch")
	}
	return visual, nil
}

// lineBounds measures one fitted region line with the bundled face: the shaped
// box at the line's size, for centring and the manifest. The fit itself was the
// metrics table's (CDS-86), so no character count applies here.
func (r *Rendering) lineBounds(ctx context.Context, ws clip.MediaWorkspace, text string, role design.TypeRole) (clip.Region, error) {
	if err := r.checkCopy(text, role); err != nil {
		return clip.Region{}, err
	}
	measured, err := r.measure(ctx, ws, []string{text}, role.Weight, role.Tracking, r.family(role))
	if err != nil {
		return clip.Region{}, err
	}
	return scaled(measured[text], role.Size/100), nil
}
