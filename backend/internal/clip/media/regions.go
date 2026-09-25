package media

import (
	"context"
	"fmt"
	"math"
	"slices"
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

// sampledBounds is what CDS-44 crops an element's frames to: its own text, or
// for an intro or outro entry that draws, its whole block's text, so every
// entry of the block reaches one scrim decision.
func sampledBounds(visual declaredVisual) clip.Region {
	bounds := regionBounds(visual)
	if visual.block != nil && bounds.Width > 0 {
		return visual.block.text
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
// entry draws only its own slots and the decoration bound to them, and the one
// that paints the rules paints them and the block's own decoration at the
// block's positions (CLIP-147, CDS-73).
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
	visual.block = newRegionBlock(canvas, kind, id, block)
	draw := regionDraw{view: &visual.region}
	part := func(e design.Element, rotated bool) {
		e.StartMS, e.EndMS = visual.manifest.StartMS, visual.manifest.EndMS
		if rotated {
			e.Rotate = block.Rotate.Deg
		}
		visual.manifest.Parts = append(visual.manifest.Parts, e)
	}
	shape := func(s design.RegionShape) error {
		box := clip.Region(s.Box)
		if !inside(box, canvas.Safe) {
			return elementProblem(visual.text, "safe_area")
		}
		fill, fillAlpha := "none", "1"
		if s.Fill != "" {
			fill, fillAlpha = s.Fill, trimmed(s.FillAlpha)
		}
		stroke, strokeAlpha := "none", "1"
		if s.Stroke != "" {
			stroke, strokeAlpha = s.Stroke, trimmed(s.StrokeAlpha)
		}
		drawn := overlay.Shape{Box: overlayBox(box, s.Radius, fill, fillAlpha), Circle: s.Circle, Stroke: stroke, StrokeOpacity: strokeAlpha, StrokeWidth: s.StrokeWidth, Shadow: s.Shadow}
		if s.Circle {
			drawn.X, drawn.Y = box.X+box.Width/2, box.Y+box.Height/2
		}
		draw.shape(drawn, s.Rotated)
		colour, alpha := regionShapePaint(s)
		part(design.Element{Kind: "plate", Rule: s.Kind, Fill: colour, Opacity: alpha, Region: design.Bounds(box)}, s.Rotated)
		return nil
	}
	lines := 0
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
			text := overlayText(role, line.Text, 0, line.Baseline, fill, trimmed(alpha))
			paintRegionText(&text, slot.Spec, fill, alpha)
			var box clip.Region
			if line.Arc != nil {
				// Along the ring (CDS-99): the arc sets the glyphs, so the slot's
				// band is the part's box and nothing is centred by measure.
				if err := r.checkCopy(line.Text, role); err != nil {
					return visual, declaredRoleError(visual.text, err)
				}
				box = clip.Region(slot.Box)
				draw.arc(overlay.ArcText{Text: text, ID: fmt.Sprintf("arc%d", index), Path: arcPath(*line.Arc)}, line.Rotated)
			} else {
				bounds, err := r.lineBounds(ctx, ws, line.Text, role)
				if err != nil {
					return visual, declaredRoleError(visual.text, err)
				}
				x := line.X - bounds.Width/2
				if line.Align == "left" {
					x = line.X
				}
				box = clip.Region{X: x, Y: line.Baseline + bounds.Y, Width: bounds.Width, Height: bounds.Height}
				text.X = box.X - bounds.X
				draw.line(text, line.Rotated)
			}
			if !inside(box, canvas.Safe) {
				return visual, elementProblem(visual.text, "safe_area")
			}
			part(design.Element{Kind: "copy", Slot: index + 1, Line: k, Text: line.Text, FontSize: role.Size, Floor: role.Floor, Fill: fill, Opacity: alpha, BaselineY: line.Baseline, GlyphOffsetY: box.Y - line.Baseline, Region: design.Bounds(box)}, line.Rotated)
			lines++
		}
		for _, s := range block.Shapes {
			if s.Slot == index {
				if err := shape(s); err != nil {
					return visual, err
				}
			}
		}
	}
	// The preset's own rules and block decoration are painted once for the
	// region, by the first entry that has a line to paint (CDS-73, CLIP-147).
	if placement.Rules && lines > 0 {
		white := design.Color["text_white"].Hex
		turned := block.Rotate.Deg != 0
		for _, rule := range block.Rules {
			box := clip.Region(rule.Box)
			draw.rule(overlayBox(box, 0, white, trimmed(rule.Alpha)), turned)
			part(design.Element{Kind: "plate", Rule: rule.Kind, Fill: white, Opacity: rule.Alpha, Region: design.Bounds(box)}, turned)
		}
		for _, s := range block.Shapes {
			if s.Slot < 0 {
				if err := shape(s); err != nil {
					return visual, err
				}
			}
		}
		visual.block.owner = true
	}
	draw.finish(block.Rotate)
	visual.manifest.Region = regionBounds(visual)
	for _, part := range visual.manifest.Parts {
		visual.manifest.Region = unionRegion(visual.manifest.Region, clip.Region(part.Region))
	}
	if err := design.VerifyRegion(kind, id, ratio, regionRows, placement.Offset, placement.Drawn, placement.Rules, visual.manifest.Parts); err != nil {
		return visual, elementProblem(visual.text, "preset_mismatch")
	}
	return visual, nil
}

// paintRegionText gives a slot's line its preset paint: a white outline with no
// fill (CDS-92), or the fill under `stroke.dark` at the token's or the preset's
// own width (CDS-94).
func paintRegionText(t *overlay.Text, spec design.RegionSlotSpec, fill string, alpha float64) {
	t.Stroke, t.StrokeOpacity = "none", "1"
	switch {
	case spec.Outline > 0:
		t.Fill, t.Opacity = "none", "1"
		t.Stroke, t.StrokeOpacity, t.StrokeWidth = fill, trimmed(alpha), spec.Outline
	case spec.Stroke == "text" || spec.Stroke == "small":
		t.Stroke, t.StrokeOpacity = paint("stroke_dark")
		t.StrokeWidth = design.Spacing.StrokeText
		if spec.Stroke == "small" {
			t.StrokeWidth = design.Spacing.StrokeSmall
		}
		if spec.StrokeWidth > 0 {
			t.StrokeWidth = spec.StrokeWidth
		}
	}
	t.Shadow = spec.Shadow != ""
}

// regionShapePaint is the colour a decoration part records: its fill, or its
// stroke for an outline.
func regionShapePaint(s design.RegionShape) (string, float64) {
	if s.Fill != "" {
		return s.Fill, s.FillAlpha
	}
	return s.Stroke, s.StrokeAlpha
}

// arcPath runs left to right over the top of the circle, or under it so the
// glyphs stand upright (CDS-99).
func arcPath(a design.RegionArc) string {
	sweep := 1
	if a.Lower {
		sweep = 0
	}
	return fmt.Sprintf("M %.3f %.3f A %.3f %.3f 0 0 %d %.3f %.3f", a.CX-a.R, a.CY, a.R, a.R, sweep, a.CX+a.R, a.CY)
}

// regionDraw sorts an entry's drawables into the upright layer and the one
// group the block's rotation turns.
type regionDraw struct {
	view *overlay.RegionView
	turn overlay.Turn
	any  bool
}

func (d *regionDraw) line(t overlay.Text, rotated bool) {
	if rotated {
		d.turn.Lines, d.any = append(d.turn.Lines, t), true
		return
	}
	d.view.Lines = append(d.view.Lines, t)
}
func (d *regionDraw) arc(a overlay.ArcText, rotated bool) {
	if rotated {
		d.turn.Arcs, d.any = append(d.turn.Arcs, a), true
		return
	}
	d.view.Arcs = append(d.view.Arcs, a)
}
func (d *regionDraw) shape(s overlay.Shape, rotated bool) {
	if rotated {
		d.turn.Shapes, d.any = append(d.turn.Shapes, s), true
		return
	}
	d.view.Shapes = append(d.view.Shapes, s)
}
func (d *regionDraw) rule(b overlay.Box, rotated bool) {
	if rotated {
		d.turn.Rules, d.any = append(d.turn.Rules, b), true
		return
	}
	d.view.Rules = append(d.view.Rules, b)
}
func (d *regionDraw) finish(r design.RegionRotation) {
	if d.any {
		d.turn.Deg, d.turn.X, d.turn.Y = r.Deg, r.CX, r.CY
		d.view.Turn = &d.turn
	}
}

// Region scrim geometry (CDS-32): a centred block's ellipse is this wide and
// this much taller than the block, and an edge block's band reaches this far
// past it.
const (
	regionRadialWidth  = 1360
	regionRadialExtra  = 560
	regionEdgeOverhang = 150
)

// regionBlock is what one entry knows of its whole block: the text bounds every
// entry of the block samples, so they all reach one scrim decision (CDS-44);
// the scrim that decision draws, painted only by the block's owner (CDS-32);
// and the filled decoration a line may stand on, for its effective background.
type regionBlock struct {
	kind, id string
	text     clip.Region
	scrim    blockScrim
	fills    []design.RegionShape
	owner    bool
}

type blockScrim struct {
	edge   string
	region clip.Region
	paint  design.ScrimPaint
}

func newRegionBlock(canvas clip.Canvas, kind, id string, layout design.RegionLayout) *regionBlock {
	b := &regionBlock{kind: kind, id: id}
	for i, slot := range layout.Slots {
		if i == 0 {
			b.text = clip.Region(slot.Box)
		} else {
			b.text = unionRegion(b.text, clip.Region(slot.Box))
		}
	}
	for _, s := range layout.Shapes {
		if s.Fill != "" && s.FillAlpha > 0 {
			b.fills = append(b.fills, s)
		}
	}
	all := clip.Region(layout.Bounds)
	w, h := float64(canvas.Width), float64(canvas.Height)
	switch layout.Scrim {
	case "top":
		b.scrim = blockScrim{"top", clip.Region{Width: w, Height: all.Y + all.Height + regionEdgeOverhang}, design.Scrim["top"]}
	case "bottom":
		y := all.Y - regionEdgeOverhang
		b.scrim = blockScrim{"bottom", clip.Region{Y: y, Width: w, Height: h - y}, design.Scrim["bottom"]}
	default:
		height := all.Height + regionRadialExtra
		b.scrim = blockScrim{"radial", clip.Region{X: all.X + all.Width/2 - regionRadialWidth/2, Y: all.Y + all.Height/2 - height/2, Width: regionRadialWidth, Height: height}, design.Scrim["radial"]}
	}
	return b
}

// alpha is the scrim's opacity at a point.
func (s blockScrim) alpha(x, y float64) float64 {
	if s.edge != "radial" {
		return scrimAlpha(s.paint, s.region, clip.Region{Y: y})
	}
	rx, ry := s.region.Width/2, s.region.Height/2
	d := math.Hypot((x-s.region.X-rx)/rx, (y-s.region.Y-ry)/ry)
	p := s.paint
	switch {
	case d >= 1:
		return p.To
	case d <= p.MidAt:
		return p.From + (p.Mid-p.From)*d/p.MidAt
	default:
		return p.Mid + (p.To-p.Mid)*(d-p.MidAt)/(1-p.MidAt)
	}
}

// draw puts the scrim into the view and returns the part's box, the drawn
// shape clipped to the canvas.
func (s blockScrim) draw(canvas clip.Canvas, view *overlay.RegionView) clip.Region {
	p := s.paint
	if s.edge == "radial" {
		view.Radial = &overlay.Radial{CX: s.region.X + s.region.Width/2, CY: s.region.Y + s.region.Height/2, RX: s.region.Width / 2, RY: s.region.Height / 2, Fill: p.Hex, From: trimmed(p.From), Mid: trimmed(p.Mid), MidAt: trimmed(p.MidAt), To: trimmed(p.To)}
	} else {
		view.Scrim = &overlay.Scrim{Box: overlayBox(s.region, 0, p.Hex, ""), From: trimmed(p.From), To: trimmed(p.To)}
	}
	x, y := max(0, s.region.X), max(0, s.region.Y)
	right, bottom := min(float64(canvas.Width), s.region.X+s.region.Width), min(float64(canvas.Height), s.region.Y+s.region.Height)
	return clip.Region{X: x, Y: y, Width: right - x, Height: bottom - y}
}

// under reports whether a filled decoration covers a point.
func under(s design.RegionShape, x, y float64) bool {
	b := s.Box
	if s.Circle {
		r := b.Width / 2
		return math.Hypot(x-b.X-r, y-b.Y-r) <= r
	}
	return x >= b.X && x <= b.X+b.Width && y >= b.Y && y <= b.Y+b.Height
}

// applyRegionGround gives a region entry what its sampled ground implies: the
// block's scrim when the ground is bright, drawn by the owner alone, and each
// line's effective background through that scrim, any plate, pill or ring fill
// under it and its own stroke (CDS-32, CDS-44).
func applyRegionGround(canvas clip.Canvas, visual *declaredVisual) {
	b := visual.block
	visual.manifest.Parts = slices.DeleteFunc(slices.Clone(visual.manifest.Parts), func(p design.Element) bool { return p.Kind == "scrim" })
	visual.region.Scrim, visual.region.Radial = nil, nil
	scrimmed := visual.ground.Scrim()
	if scrimmed && b.owner {
		box := b.scrim.draw(canvas, &visual.region)
		visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "scrim", Region: design.Bounds(box), StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
	}
	preset, _ := design.Region(b.kind, b.id)
	slots := preset.Slots()
	stroke := design.Color["stroke_dark"]
	for i := range visual.manifest.Parts {
		p := &visual.manifest.Parts[i]
		if p.Kind != "copy" {
			continue
		}
		x, y := p.Region.X+p.Region.Width/2, p.Region.Y+p.Region.Height/2
		ground := visual.ground.Hex()
		if scrimmed {
			if a := b.scrim.alpha(x, y); a > 0 {
				if washed, ok := design.Over(b.scrim.paint.Hex, a, ground); ok {
					ground = washed
				}
			}
		}
		for _, f := range b.fills {
			if under(f, x, y) {
				if on, ok := design.Over(f.Fill, f.FillAlpha, ground); ok {
					ground = on
				}
			}
		}
		if p.Slot >= 1 && p.Slot <= len(slots) && slots[p.Slot-1].Outline == 0 && (slots[p.Slot-1].Stroke == "text" || slots[p.Slot-1].Stroke == "small") {
			if on, ok := design.Over(stroke.Hex, stroke.Alpha, ground); ok {
				ground = on
			}
		}
		p.Background = ground
		p.ContrastNotice = !design.Legible(design.Manifest{*p})
	}
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
