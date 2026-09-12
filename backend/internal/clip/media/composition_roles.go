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

func (r *Rendering) layoutDeclaredRole(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual) (declaredVisual, error) {
	e := visual.text.Resolved.Element
	// Named text styles describe caption visuals. Other roles have their own
	// CDS typography/plate contract; incompatible authored choices need editing.
	if e.Style != "auto" {
		return visual, elementProblem(visual.text, "invalid_style")
	}
	switch e.Role {
	case "badge":
		return r.layoutDeclaredBadge(ctx, ws, canvas, ratio, visual)
	case "info":
		return r.layoutDeclaredInfo(ctx, ws, canvas, ratio, visual)
	case "hook", "ending":
		return r.layoutDeclaredCard(ctx, ws, canvas, ratio, visual)
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
	if strings.TrimSpace(text) == "" || strings.Contains(text, "\n") {
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
	visual.manifest.Parts = clip.Manifest{{Kind: "badge", Text: text, Region: design.Region(box), FontSize: design.Type["badge"].Size, Fill: design.Color["text_white"].Hex, Background: design.Color["badge_ad"].Hex, StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS}}
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
	pad, gap := design.Spacing.PadChip, design.Spacing.GapChip
	bounds := make([]clip.Region, len(rows))
	width, height := 0.0, 0.0
	horizontal := len(rows) == 2 && rows[0].Role == "label" && rows[1].Role == "caption"
	for i, row := range rows {
		role, known := design.Type[row.Role]
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
	fill, alpha := paint("ink_900")
	plate := overlayBox(box, box.Height/2, fill, alpha)
	visual.info = overlay.CopyView{Canvas: overlay.Canvas{Width: canvas.Width, Height: canvas.Height}, Plate: &plate}
	visual.manifest.Parts = clip.Manifest{{Kind: "plate", Region: design.Region(box), Background: fill, StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS}}
	x, y := box.X+pad.H, box.Y+pad.V
	for i, row := range rows {
		role := design.Type[row.Role]
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
		visual.info.Lines = append(visual.info.Lines, overlayText(role, row.Text, lineX-b.X, baselineY, colour, opacity))
		visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "copy", Text: row.Text, Region: design.Region{X: lineX, Y: baselineY + b.Y, Width: b.Width, Height: b.Height}, FontSize: role.Size, Fill: colour, Background: fill, StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
		if horizontal {
			x += b.Width + gap
		} else {
			y += math.Max(b.Height, role.Size) + design.Spacing.GapStack
		}
	}
	return visual, nil
}

func (r *Rendering) layoutDeclaredCard(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, visual declaredVisual) (declaredVisual, error) {
	text := visual.text
	rows := text.Resolved.Rows
	if len(rows) == 0 {
		role := "body"
		if text.Resolved.Element.Role == "hook" {
			role = "hook"
		}
		rows = []composition.ResolvedRow{{Role: role, Text: text.Resolved.Text}}
	}
	kind := "hook"
	if text.Resolved.Element.Role == "ending" {
		kind = "end"
	}
	card := cardLayout{Kind: kind, Align: text.Resolved.Element.Align, StartMS: text.Resolved.StartMS, EndMS: text.Resolved.EndMS, Accent: text.Accent}
	for rowIndex, row := range rows {
		role, known := design.Type[row.Role]
		if !known {
			return visual, elementProblem(text, "invalid_row_role")
		}
		if row.Role == "hook" {
			geometry, _ := design.Layout(ratio)
			role.Size, role.Min = geometry.HookSize, math.Min(role.Min, geometry.HookSize)
		}
		lines := []string{row.Text}
		if strings.Contains(row.Text, "\n") {
			lines = strings.Split(row.Text, "\n")
		} else if row.Role == "hook" && design.Chars(row.Text) > role.Chars && design.Chars(row.Text) <= 2*role.Chars {
			lines = hookLines(row.Text)
		}
		if row.Role == "hook" && len(lines) > 2 {
			return visual, elementProblem(text, "copy_limit")
		}
		for _, line := range lines {
			if strings.TrimSpace(line) == "" || role.Chars > 0 && design.Chars(line) > role.Chars {
				return visual, elementProblem(text, "copy_limit")
			}
			colour := design.Color["text_white"]
			chip := kind == "hook" && row.Role == "label" && text.Accent != ""
			if kind == "hook" && row.Role == "body" || kind == "end" && row.Role == "label" {
				colour = design.Color["text_muted"]
			}
			if chip {
				colour = design.Color["ink_900"]
			}
			if kind == "end" && row.Role == "label" && rowIndex == len(rows)-1 && text.Accent != "" {
				colour.Hex, colour.Alpha = design.Accent[text.Accent], 1
			}
			card.Lines = append(card.Lines, cardLine{Text: line, Role: role, Fill: colour.Hex, Alpha: colour.Alpha, Chip: chip})
		}
	}
	card, err := r.measureCard(ctx, ws, canvas, ratio, card)
	if err != nil {
		return visual, declaredRoleError(text, err)
	}
	for _, bounds := range card.Bounds {
		if bounds.Width > card.Region.Width-2*cardPadding {
			return visual, elementProblem(text, "copy_limit")
		}
	}
	if text.Resolved.Element.Position != "auto" {
		box, _, err := rolePosition(canvas, ratio, text.Resolved.Element, card.Region.Width, card.Region.Height)
		if err != nil {
			return visual, elementProblem(text, "safe_area")
		}
		card.Region = box
	}
	visual.card, visual.manifest.Region = card, card.Region
	visual.manifest.Parts = card.Elements(0)
	return visual, nil
}
