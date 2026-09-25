package media

import (
	"context"
	"math"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

// elements is every resolved entry of the plan, which a region entry is laid
// out among (CDS-87); other roles ignore it.
func (r *Rendering) layoutDeclaredRole(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual, selection composition.DesignSelection, placement clip.RegionPlacement, elements []composition.ResolvedElement) (declaredVisual, error) {
	e := visual.text.Resolved.Element
	switch e.Role {
	case "badge":
		return r.layoutDeclaredBadge(ctx, ws, canvas, ratio, visual)
	case "info":
		return r.layoutDeclaredInfo(ctx, ws, canvas, ratio, visual)
	case "hook", "ending":
		kind := "intro"
		if e.Role == "ending" {
			kind = "outro"
		}
		rows := clip.RegionRows(regionElements(elements, visual.text.Resolved), selection, kind)
		return r.layoutDeclaredRegion(ctx, ws, canvas, ratio, visual, kind, clip.RegionPresetID(selection, kind), placement, rows)
	default:
		return visual, elementProblem(visual.text, "invalid_role")
	}
}

func rolePosition(canvas clip.Canvas, ratio string, element composition.Element, width, height float64) (clip.Region, string, error) {
	position := element.Position
	l, _ := design.Layout(ratio)
	if position == "auto" && (element.Role == "badge" || element.Role == "info") {
		position = "header"
	}
	if position == "header" {
		x := l.Anchor.Left
		if element.Role == "badge" {
			x = l.Badge.Right - width
		}
		box := clip.Region{X: x, Y: l.Badge.Top, Width: width, Height: height}
		safe := canvas.Safe
		if element.Role == "badge" {
			safe.X, safe.Width = l.Anchor.Left, l.Badge.Right-l.Anchor.Left
		}
		if !inside(box, safe) {
			return box, position, clip.ErrInvalid
		}
		return box, position, nil
	}
	box, err := clip.PlaceCopy(canvas, position, element.Align, width, height)
	return box, position, err
}

func (r *Rendering) roleBounds(ctx context.Context, ws clip.MediaWorkspace, text string, role design.TypeRole) (clip.Region, error) {
	if err := r.checkCopy(text, role); err != nil {
		return clip.Region{}, err
	}
	if strings.TrimSpace(text) == "" || strings.Contains(text, "\n") || role.Chars > 0 && design.Chars(text) > role.Chars {
		return clip.Region{}, clip.ErrCopyTooLong
	}
	measured, err := r.measure(ctx, ws, []string{text}, role.Weight, role.Tracking, r.family(role))
	if err != nil {
		return clip.Region{}, err
	}
	return scaled(measured[text], role.Size/100), nil
}

func (r *Rendering) layoutDeclaredBadge(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual) (declaredVisual, error) {
	text := visual.text.Resolved.Text
	bounds, err := r.roleBounds(ctx, ws, text, design.Type["badge"])
	if err != nil {
		return visual, declaredRoleError(visual.text, err)
	}
	box, position, err := rolePosition(canvas, ratio, visual.text.Resolved.Element, math.Ceil(bounds.Width+2*design.Spacing.PadChip.H), math.Ceil(design.Type["badge"].Size+2*design.Spacing.PadChip.V))
	if err != nil {
		return visual, elementProblem(visual.text, "safe_area")
	}
	visual.furniture = furniture{Badge: box, BadgeText: text, BadgeBounds: bounds}
	visual.manifest.Region, visual.manifest.Position = box, position
	visual.manifest.Parts = clip.Manifest{{Kind: "badge", Text: text, Region: design.Bounds(box), FontSize: design.Type["badge"].Size, Fill: design.Color["text_white"].Hex, Background: design.Color["badge_ad"].Hex, StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS}}
	return visual, nil
}

func declaredRoleError(text clip.PortableText, err error) error {
	if err == clip.ErrCopyTooLong {
		return elementProblem(text, "copy_limit")
	}
	if err == clip.ErrInvalid {
		return elementProblem(text, "unsupported_glyph")
	}
	return err
}

// Information is a centered stack of authored labels and values. A literal
// without rows is a value; wrapping preserves its words and never adds a frame.
func (r *Rendering) layoutDeclaredInfo(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual) (declaredVisual, error) {
	rows := visual.text.Resolved.Rows
	if len(rows) == 0 {
		rows = []composition.ResolvedRow{{Role: "caption", Text: visual.text.Resolved.Text}}
	}
	geometry, _ := design.Layout(ratio)
	roleFor := func(name string) (design.TypeRole, bool) {
		if name != "label" && name != "caption" {
			return design.TypeRole{}, false
		}
		role := design.Type[name]
		if name == "label" {
			role.Tracking = design.Information.LabelTracking
		}
		return role, true
	}
	var wrapped []composition.ResolvedRow
	for _, row := range rows {
		if strings.TrimSpace(row.Text) == "" {
			continue
		}
		role, known := roleFor(row.Role)
		if !known {
			return visual, elementProblem(visual.text, "invalid_row_role")
		}
		fits := func(text string) bool {
			b, err := r.roleBounds(ctx, ws, text, role)
			return err == nil && b.Width <= geometry.CopyMaxWidth
		}
		if fits(row.Text) {
			wrapped = append(wrapped, row)
			continue
		}
		words := strings.Fields(row.Text)
		found := false
		if !strings.ContainsAny(row.Text, "\r\n") {
			for split := len(words) - 1; split > 0; split-- {
				a, b := strings.Join(words[:split], " "), strings.Join(words[split:], " ")
				if fits(a) && fits(b) {
					wrapped = append(wrapped, composition.ResolvedRow{Role: row.Role, Text: a}, composition.ResolvedRow{Role: row.Role, Text: b})
					found = true
					break
				}
			}
		}
		if !found {
			return visual, elementProblem(visual.text, "copy_limit")
		}
	}
	if len(wrapped) == 0 {
		return visual, nil
	}
	bounds := make([]clip.Region, len(wrapped))
	width, height := 0.0, 0.0
	for i, row := range wrapped {
		role, _ := roleFor(row.Role)
		b, err := r.roleBounds(ctx, ws, row.Text, role)
		if err != nil {
			return visual, declaredRoleError(visual.text, err)
		}
		bounds[i] = b
		width = math.Max(width, b.Width)
		height += math.Max(role.Size, b.Height)
		if i > 0 {
			height += design.Spacing.GapStack
		}
	}
	element := visual.text.Resolved.Element
	element.Align = "center"
	box, position, err := rolePosition(canvas, ratio, element, math.Ceil(width), math.Ceil(height))
	if err != nil {
		return visual, elementProblem(visual.text, "safe_area")
	}
	visual.manifest.Region, visual.manifest.Position, visual.manifest.Align = box, position, "center"
	shadow := overlayShadow("text")
	visual.info = overlay.CopyView{Canvas: overlay.Canvas{Width: canvas.Width, Height: canvas.Height}, Shadow: &shadow}
	y := box.Y
	for i, row := range wrapped {
		role, _ := roleFor(row.Role)
		b := bounds[i]
		colour := design.Color["text_white"]
		if row.Role == "label" {
			colour = design.Color["text_muted"]
		}
		glyph := clip.Region{X: box.X + (box.Width-b.Width)/2, Y: y, Width: b.Width, Height: b.Height}
		line := overlayText(role, row.Text, glyph.X-b.X, y-b.Y, colour.Hex, trimmed(colour.Alpha))
		line.Stroke, line.StrokeOpacity = paint("stroke_dark")
		line.StrokeWidth = design.Spacing.StrokeSmall
		line.Shadow = true
		visual.info.Lines = append(visual.info.Lines, line)
		visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "copy", TypeRole: row.Role, Text: row.Text, Region: design.Bounds(glyph), FontSize: role.Size, Fill: colour.Hex, Opacity: colour.Alpha, StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
		y += math.Max(role.Size, b.Height) + design.Spacing.GapStack
	}
	return visual, nil
}
