package media

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

// Measured layout becomes plain, versioned template data here. This adapter
// preserves CDS arithmetic; SVG markup belongs entirely to the asset catalog.
func overlayBox(p clip.Region, radius float64, fill, opacity string) overlay.Box {
	return overlay.Box{X: p.X, Y: p.Y, Width: p.Width, Height: p.Height, Radius: radius, Fill: fill, Opacity: opacity}
}
func overlayShadow(name string) overlay.Shadow { return shadowPaint(design.Shadow[name]) }
func shadowPaint(s design.ShadowPaint) overlay.Shadow {
	return overlay.Shadow{DX: s.DX, DY: s.DY, Deviation: s.Blur / 2, Fill: s.Hex, Opacity: trimmed(s.Alpha)}
}
func overlayText(role design.TypeRole, value string, x, y float64, fill, opacity string) overlay.Text {
	return overlay.Text{X: x, Y: y, Family: design.FontFamily(role.Face), Size: role.Size, Weight: role.Weight, Tracking: role.Tracking * role.Size, Fill: fill, Opacity: opacity, Value: value}
}

func copyView(canvas clip.Canvas, c clip.Copy, l copyLayout, ground Luminance) overlay.CopyView {
	v := overlay.CopyView{Canvas: overlay.Canvas{Width: canvas.Width, Height: canvas.Height}}
	p, s, accent := l.Region, l.Style, design.Accent[c.Accent]
	if !l.Caption.Paint.Accent {
		accent = ""
	}
	if s.Plate == "" && l.Caption.Paint.Scrim && ground.Scrim() {
		if scrim, ok := scrimFor(canvas, c.Anchor); ok {
			paint := design.Scrim[scrim.Edge]
			v.Scrim = &overlay.Scrim{Box: overlayBox(scrim.Region, 0, paint.Hex, ""), From: trimmed(paint.From), To: trimmed(paint.To)}
		}
	}
	word := accent
	if ground.AccentWhite() && !s.Highlight {
		word = design.Color["text_white"].Hex
	}
	if s.Shadow != "" {
		shadow := shadowPaint(l.Caption.Paint.Shadow)
		v.Shadow = &shadow
	}
	if s.Plate != "" {
		fill, alpha := paint(s.Plate)
		box := overlayBox(p, design.Spacing.RadiusBox, fill, alpha)
		v.Plate = &box
	}
	if accent != "" && s.Bar {
		v.Bar = &overlay.Box{X: p.X, Y: p.Y, Width: design.Spacing.BarAccent, Height: p.Height, Fill: accent}
	}
	if accent != "" && s.Dot {
		r := design.Spacing.DotAccent / 2
		v.Dot = &overlay.Circle{X: p.X + s.Padding.H + r, Y: p.Y + s.Padding.V + r, Radius: r, Fill: accent}
	}
	left, right, vertical := copyInsets(s)
	inner, top := p.Width-left-right, p.Y+vertical
	for i, line := range l.Lines {
		bounds := l.Bounds[i]
		x, y := p.X+left+(inner-bounds.Width)/2-bounds.X, top-bounds.Y
		t := overlayText(l.Role, line, x, y, l.Caption.Paint.Fill, "")
		t.Size, t.Tracking = l.FontSize, l.Role.Tracking*l.FontSize
		t.Stroke, t.StrokeOpacity = "none", "1"
		if s.Stroke != "" && l.Caption.Paint.Stroke != "" {
			t.Stroke, t.StrokeOpacity = l.Caption.Paint.Stroke, trimmed(design.Color["stroke_dark"].Alpha)
		}
		t.StrokeWidth, t.Shadow = s.StrokeWidth(), s.Shadow != ""
		if accent != "" && s.Highlight && l.Keyword.Present && l.Keyword.Line == i {
			u := design.Spacing.UnderlineMark
			t.Highlight = &overlay.Box{X: x + bounds.X + l.Keyword.Offset - u.Extend, Y: y - (u.RaiseEM+u.HeightEM)*l.FontSize, Width: l.Keyword.Width + 2*u.Extend, Height: u.HeightEM * l.FontSize, Fill: accent, Opacity: "0.9"}
		}
		if accent != "" && !s.Highlight && s.Stroke != "" && l.Keyword.Present && l.Keyword.Line == i {
			at := strings.Index(line, l.Keyword.Text)
			t.Colored, t.Prefix, t.Keyword, t.Suffix, t.Accent = true, line[:at], l.Keyword.Text, line[at+len(l.Keyword.Text):], word
		}
		v.Lines = append(v.Lines, t)
		top += bounds.Height + l.FontSize*(l.Role.LineHeight-1)
	}
	return v
}

func furnitureView(canvas clip.Canvas, f furniture) overlay.FurnitureView {
	shadow := overlayShadow("text")
	v := overlay.FurnitureView{Canvas: overlay.Canvas{Width: canvas.Width, Height: canvas.Height}, Shadow: &shadow}
	badge, badgeAlpha := paint("badge_ad")
	white, _ := paint("text_white")
	muted, mutedAlpha := paint("text_muted")
	p := f.Badge
	if f.BadgeText != "" {
		box := overlayBox(p, design.Spacing.RadiusChip, badge, badgeAlpha)
		label := overlayText(design.Type["badge"], f.BadgeText, p.X+design.Spacing.PadChip.H-f.BadgeBounds.X, p.Y+(p.Height-f.BadgeBounds.Height)/2-f.BadgeBounds.Y, white, "")
		v.Badge, v.Label = &box, &label
	}
	for _, c := range f.Chips {
		labelRole := design.Type["label"]
		labelRole.Tracking = design.Information.LabelTracking
		x := c.Region.X + c.Region.Width/2
		label := overlayText(labelRole, c.Label, x-c.LabelBounds.Width/2-c.LabelBounds.X, c.Region.Y-c.LabelBounds.Y, muted, mutedAlpha)
		value := overlayText(design.Type["caption"], c.Value, x-c.ValueBounds.Width/2-c.ValueBounds.X, c.Region.Y+math.Max(labelRole.Size, c.LabelBounds.Height)+design.Spacing.GapStack-c.ValueBounds.Y, white, "")
		label.Stroke, label.StrokeOpacity = paint("stroke_dark")
		value.Stroke, value.StrokeOpacity = label.Stroke, label.StrokeOpacity
		label.StrokeWidth, value.StrokeWidth = design.Spacing.StrokeSmall, design.Spacing.StrokeSmall
		label.Shadow, value.Shadow = true, true
		v.Chips = append(v.Chips, overlay.Chip{Label: label, Value: value})
	}
	return v
}

func loadOverlays(directory string) (*overlay.Catalog, error) {
	var catalog *overlay.Catalog
	var err error
	if directory == "" {
		catalog, err = overlay.Builtin()
	} else {
		info, e := os.Lstat(directory)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid clip overlay directory")
		}
		catalog, err = overlay.Load(os.DirFS(directory))
	}
	if err != nil {
		return nil, err
	}
	// Exercise every current binding before admitting media work. Asset loading
	// does not change the product's approved style IDs or select a new design.
	for _, preset := range catalog.Presets() {
		if err := catalog.Validate(preset.ID, overlayProbe(preset.View)); err != nil {
			return nil, err
		}
	}
	// Every approved style (CDS-80) has to have a template bound to it: an
	// unbound id would fail at the rasterisation, long after a layout took it.
	for _, style := range design.CaptionStyles() {
		if _, err := catalog.Render("copy."+style.ID, overlayProbe("copy-v1")); err != nil {
			return nil, err
		}
	}
	for binding, view := range map[string]string{"furniture": "furniture-v1", "region": "region-v2", "info": "info-v1"} {
		if _, err := catalog.Render(binding, overlayProbe(view)); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

// Populated optional slots catch asset/view mismatches at startup, including
// presets not currently selected by any binding. Runtime values are checked too.
func overlayProbe(view string) any {
	canvas := overlay.Canvas{Width: 1080, Height: 1920}
	box := overlay.Box{Width: 100, Height: 100, Fill: "#111111", Opacity: "1"}
	text := overlay.Text{Family: fontFamily, Size: 56, Weight: 700, Fill: "#FFFFFF", Opacity: "1", Stroke: "none", StrokeOpacity: "1", Value: "한글", Colored: true, Keyword: "한글", Accent: "#FF6B57", Shadow: true, Highlight: &box}
	switch view {
	case "copy-v1":
		return overlay.CopyView{Canvas: canvas, Plate: &box, Bar: &box, Dot: &overlay.Circle{Radius: 1, Fill: "#111111"}, Shadow: &overlay.Shadow{Fill: "#111111", Opacity: "1"}, Scrim: &overlay.Scrim{Box: box, From: "0", To: "1"}, Lines: []overlay.Text{text}}
	case "info-v1":
		return overlayProbe("copy-v1").(overlay.CopyView)
	case "furniture-v1":
		return overlay.FurnitureView{Canvas: canvas, Badge: &box, Label: &text, Chips: []overlay.Chip{{Box: box, Label: text, Value: text}}}
	case "region-v1":
		return overlay.RegionView{CopyView: overlayProbe("copy-v1").(overlay.CopyView), Rules: []overlay.Box{box}}
	default:
		shapes := []overlay.Shape{{Box: box, Stroke: "#FFFFFF", StrokeOpacity: "1", StrokeWidth: 2, Shadow: true}, {Box: box, Circle: true, Stroke: "none", StrokeOpacity: "1"}}
		arcs := []overlay.ArcText{{Text: text, ID: "arc0", Path: "M 0 100 A 100 100 0 0 1 200 100"}}
		v := overlayProbe("region-v1").(overlay.RegionView)
		v.Radial = &overlay.Radial{RX: 1, RY: 1, Fill: "#000000", From: "0.5", Mid: "0.3", MidAt: "0.55", To: "0"}
		v.Shapes, v.Arcs = shapes, arcs
		v.Turn = &overlay.Turn{Deg: -4, Rules: v.Rules, Shapes: shapes, Lines: v.Lines, Arcs: []overlay.ArcText{{Text: text, ID: "arc1", Path: arcs[0].Path}}}
		return v
	}
}

// Rapid cues retain only their painted region, avoiding a full canvas per input.
// Scrims occupy the canvas and must keep that extent for custom unplated styles.
func copyCrop(canvas clip.Canvas, c clip.Copy, l copyLayout, ground Luminance) clip.Region {
	if c.Pace != "rapid" || ground.Scrim() {
		return clip.Region{Width: float64(canvas.Width), Height: float64(canvas.Height)}
	}
	s := l.Caption.Paint.Shadow
	pad := math.Ceil(2*s.Blur + math.Max(math.Abs(s.DX), math.Abs(s.DY)) + l.Style.StrokeWidth() + 2)
	p := l.Region
	x, y := math.Max(0, math.Floor(p.X-pad)), math.Max(0, math.Floor(p.Y-pad))
	right, bottom := math.Min(float64(canvas.Width), math.Ceil(p.X+p.Width+pad)), math.Min(float64(canvas.Height), math.Ceil(p.Y+p.Height+pad))
	return clip.Region{X: x, Y: y, Width: right - x, Height: bottom - y}
}
