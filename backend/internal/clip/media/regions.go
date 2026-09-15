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

func (r *Rendering) layoutDeclaredRegion(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual, kind, id string) (declaredVisual, error) {
	preset, ok := design.Region(kind, id)
	if !ok {
		return visual, elementProblem(visual.text, "invalid_design")
	}
	rows := visual.text.Resolved.Rows
	if len(rows) == 0 {
		rows = []composition.ResolvedRow{{Text: visual.text.Resolved.Text}}
	}
	if len(rows) > len(preset.Slots) {
		return visual, elementProblem(visual.text, "copy_limit")
	}
	geometry, _ := design.Layout(ratio)
	shadow := overlayShadow("text")
	visual.region = overlay.RegionView{CopyView: overlay.CopyView{Canvas: overlay.Canvas{Width: canvas.Width, Height: canvas.Height}, Shadow: &shadow}}
	texts := make([]string, len(rows))
	for i, row := range rows {
		texts[i] = row.Text
		if strings.ContainsAny(row.Text, "\r\n") {
			return visual, elementProblem(visual.text, "copy_limit")
		}
		if strings.TrimSpace(row.Text) == "" {
			continue
		}
		slot := preset.Slots[i]
		role := design.RegionType(slot, ratio)
		bounds, err := r.roleBounds(ctx, ws, row.Text, role)
		if err != nil {
			return visual, declaredRoleError(visual.text, err)
		}
		baseline := design.RegionBaseline(preset, ratio, slot.Y)
		box := clip.Region{X: geometry.Anchor.Center - bounds.Width/2, Y: baseline + bounds.Y, Width: bounds.Width, Height: bounds.Height}
		if !inside(box, canvas.Safe) {
			return visual, elementProblem(visual.text, "safe_area")
		}
		colour := design.Color[slot.Fill]
		if slot.Alpha != nil {
			colour.Alpha *= *slot.Alpha
		}
		line := overlayText(role, row.Text, box.X-bounds.X, baseline, colour.Hex, trimmed(colour.Alpha))
		line.Stroke, line.StrokeOpacity = "none", "1"
		if slot.Stroke == "text" || slot.Stroke == "small" {
			line.Stroke, line.StrokeOpacity = paint("stroke_dark")
			line.StrokeWidth = design.Spacing.StrokeText
			if slot.Stroke == "small" {
				line.StrokeWidth = design.Spacing.StrokeSmall
			}
		}
		line.Shadow = slot.Shadow != ""
		visual.region.Lines = append(visual.region.Lines, line)
		visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "copy", Slot: i + 1, Text: row.Text, FontSize: role.Size, Fill: colour.Hex, Opacity: colour.Alpha, BaselineY: baseline, GlyphOffsetY: bounds.Y, Region: design.Bounds(box), StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
	}
	if len(visual.region.Lines) > 0 {
		for _, line := range preset.Rules {
			rule := design.Rules[line.Kind]
			box := clip.Region{X: geometry.Anchor.Center - rule.Width/2, Y: design.RegionBaseline(preset, ratio, line.Y), Width: rule.Width, Height: rule.Height}
			visual.region.Rules = append(visual.region.Rules, overlayBox(box, 0, design.Color["text_white"].Hex, trimmed(rule.Alpha)))
			visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "plate", Rule: line.Kind, Fill: design.Color["text_white"].Hex, Opacity: rule.Alpha, Region: design.Bounds(box), StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
		}
	}
	visual.manifest.Region = regionBounds(visual)
	for _, part := range visual.manifest.Parts {
		visual.manifest.Region = unionRegion(visual.manifest.Region, clip.Region(part.Region))
	}
	if err := design.VerifyRegion(kind, id, ratio, texts, visual.manifest.Parts); err != nil {
		return visual, elementProblem(visual.text, "preset_mismatch")
	}
	return visual, nil
}
