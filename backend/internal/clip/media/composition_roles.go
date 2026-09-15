package media

import (
	"context"
	"math"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

func (r *Rendering) layoutDeclaredRole(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual, selection ...composition.DesignSelection) (declaredVisual, error) {
	e := visual.text.Resolved.Element
	switch e.Role {
	case "badge":
		return r.layoutDeclaredBadge(ctx, ws, canvas, ratio, visual)
	case "info":
		return r.layoutDeclaredInfo(ctx, ws, canvas, ratio, visual)
	case "hook", "ending":
		choice := composition.DefaultDesign()
		if len(selection) > 0 {
			choice = selection[0]
		}
		kind, id := "intro", choice.Intro
		if e.Role == "ending" {
			kind, id = "outro", choice.Outro
		}
		return r.layoutDeclaredRegion(ctx, ws, canvas, ratio, visual, kind, id)
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
		x := l.Chip.X
		if element.Role == "badge" {
			x = l.Badge.Right - width
		}
		box := clip.Region{X: x, Y: l.Badge.Top, Width: width, Height: height}
		safe := canvas.Safe
		if element.Role == "badge" {
			safe.X, safe.Width = l.Chip.X, l.Badge.Right-l.Chip.X
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
	box, position, err := rolePosition(canvas, ratio, visual.text.Resolved.Element, math.Ceil(bounds.Width+2*badgePadH), math.Ceil(math.Max(bounds.Height, design.Type["badge"].Size)+2*badgePadV))
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

func (r *Rendering) layoutDeclaredInfo(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual) (declaredVisual, error) {
	rows := visual.text.Resolved.Rows
	if len(rows) == 0 {
		rows = []composition.ResolvedRow{{Role: "caption", Text: visual.text.Resolved.Text}}
	}
	rows = slices.Clone(rows)
	variant := "compact"
	frame := design.InfoFrames[variant]
	// A single short name uses t.title. Longer exact content keeps the
	// compact frame and its existing word wrapping without rewriting words.
	if len(rows) == 1 && rows[0].Role != "label" {
		title := design.Type["title"]
		title.Face = design.InfoFrames["emphasis"].Face
		b, err := r.roleBounds(ctx, ws, rows[0].Text, title)
		geometry, _ := design.Layout(ratio)
		if err == nil && b.Width+2*design.InfoFrames["emphasis"].Padding.H <= geometry.Chip.MaxWidth {
			variant = "emphasis"
			frame = design.InfoFrames[variant]
			rows[0].Role = frame.Type
		}
	}
	roleFor := func(name string) (design.TypeRole, bool) {
		role, ok := design.Type[name]
		role.Face = frame.Face
		return role, ok
	}
	// CDS-20 permits two word-wrapped lines. Preserve every authored word;
	// a physical line break is layout, never a shortened replacement fact.
	geometry, _ := design.Layout(ratio)
	var wrapped []composition.ResolvedRow
	for _, row := range rows {
		role, known := roleFor(row.Role)
		if !known {
			return visual, elementProblem(visual.text, "invalid_row_role")
		}
		fits := func(text string) bool {
			bounds, err := r.roleBounds(ctx, ws, text, role)
			return err == nil && bounds.Width <= geometry.Chip.MaxWidth-2*frame.Padding.H
		}
		if fits(row.Text) {
			wrapped = append(wrapped, row)
			continue
		}
		words := strings.Fields(row.Text)
		found := false
		if !strings.Contains(row.Text, "\n") {
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
	rows = wrapped
	pad, gap := frame.Padding, design.Spacing.GapChip
	bounds := make([]clip.Region, len(rows))
	width, height := 0.0, 0.0
	horizontal := len(rows) == 2 && rows[0].Role == "label" && rows[1].Role == "caption"
	for i, row := range rows {
		role, known := roleFor(row.Role)
		if !known {
			return visual, elementProblem(visual.text, "invalid_row_role")
		}
		box, err := r.roleBounds(ctx, ws, row.Text, role)
		if err != nil {
			return visual, declaredRoleError(visual.text, err)
		}
		bounds[i] = box
		if horizontal {
			width += box.Width
			height = math.Max(height, math.Max(box.Height, role.Size))
		} else {
			width = math.Max(width, box.Width)
			height += math.Max(box.Height, role.Size)
		}
		if i > 0 {
			if horizontal {
				width += gap
			} else {
				height += design.Spacing.GapStack
			}
		}
	}
	l, _ := design.Layout(ratio)
	width, height = math.Ceil(width+2*pad.H), math.Ceil(height+2*pad.V)
	if width > l.Chip.MaxWidth {
		return visual, elementProblem(visual.text, "copy_limit")
	}
	box, position, err := rolePosition(canvas, ratio, visual.text.Resolved.Element, width, height)
	if err != nil {
		return visual, elementProblem(visual.text, "safe_area")
	}
	visual.manifest.Region, visual.manifest.Position = box, position
	visual.infoVariant = variant
	visual.info = overlay.InfoView{CopyView: overlay.CopyView{Canvas: overlay.Canvas{Width: canvas.Width, Height: canvas.Height}}, Frame: overlayBox(box, 0, "", ""), Right: box.X + box.Width, Bottom: box.Y + box.Height}
	fill := ""
	if frame.Plate != "" {
		colour, alpha := paint(frame.Plate)
		plate := overlayBox(box, design.Spacing.RadiusBox, colour, alpha)
		visual.info.Plate = &plate
		// Certify the actual worst-case composite, including muted label alpha.
		fill, _ = design.Over(colour, design.Color[frame.Plate].Alpha, "#FFFFFF")
		visual.manifest.Parts = clip.Manifest{{Kind: "plate", Region: design.Bounds(box), Background: fill, StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS}}
	}
	if frame.Shadow != "" {
		shadow := overlayShadow(frame.Shadow)
		visual.info.Shadow = &shadow
	}
	x, y := box.X+pad.H, box.Y+pad.V
	for i, row := range rows {
		role, _ := roleFor(row.Role)
		b := bounds[i]
		colour, opacity := paint("text_white")
		if row.Role == "label" {
			colour, opacity = paint("text_muted")
		}
		baselineY := y - b.Y
		lineX := x
		if !horizontal {
			room := box.Width - 2*pad.H - b.Width
			if visual.text.Resolved.Element.Align == "center" {
				lineX += room / 2
			} else if visual.text.Resolved.Element.Align == "right" {
				lineX += room
			}
		}
		if horizontal {
			baselineY = box.Y + (box.Height-b.Height)/2 - b.Y
		}
		line := overlayText(role, row.Text, lineX-b.X, baselineY, colour, opacity)
		line.Stroke, line.StrokeOpacity = "none", "1"
		if frame.Stroke != "" {
			line.Stroke, line.StrokeOpacity = paint("stroke_dark")
			line.StrokeWidth = design.Spacing.StrokeText
			line.Shadow = frame.Shadow != ""
		}
		visual.info.Lines = append(visual.info.Lines, line)
		if row.Role == "label" {
			colour, _ = design.Over(colour, design.Color["text_muted"].Alpha, fill)
		}
		visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "copy", Text: row.Text, Region: design.Bounds{X: lineX, Y: baselineY + b.Y, Width: b.Width, Height: b.Height}, FontSize: role.Size, Fill: colour, Background: fill, StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
		if horizontal {
			x += b.Width + gap
		} else {
			y += math.Max(b.Height, role.Size) + design.Spacing.GapStack
		}
	}
	return visual, nil
}
