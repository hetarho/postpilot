package clip

import (
	"slices"

	"github.com/postpilot/backend/internal/clip/design"
)

// OwnerCaption is what the owner set on one caption in ② (CDS-82, CLIP-143):
// where they put it, how big they made it and which approved style they gave
// it. Each part is absent on its own — a nil Position is automatic placement, a
// zero Size the style's own and an empty Style the project's default — so
// resizing a caption the owner never moved does not also pin its position.
type OwnerCaption struct {
	Position *CaptionPlacement
	Size     int
	Style    string
}

// Placed reports whether automatic placement is out of this caption's way: the
// owner's own position replaces that result and is never re-run over (CDS-38).
func (o OwnerCaption) Placed() bool { return o.Position != nil }

// ValidateOwnerCaption admits what ② may set on one caption and refuses the
// rest where it is written (CDS-82, CLIP-143). A style outside the project's
// allowed set is an authoring error, and so is a size below CDS-3's floor for
// that style's role — or above the size the role is set at, which V2 refuses on
// the way out. The POSITION is clamped rather than refused: CDS-82 clamps
// position only, and an owner who drags past the edge means the edge.
func ValidateOwnerCaption(in OwnerCaption, role, ratio string, allowed []string) (OwnerCaption, error) {
	if role != "caption" {
		if in != (OwnerCaption{}) {
			return OwnerCaption{}, ErrInvalid
		}
		return OwnerCaption{}, nil
	}
	rule := design.Caption()
	if in.Style != "" {
		style, ok := design.CaptionRule(in.Style)
		if !ok || !slices.Contains(allowed, in.Style) {
			return OwnerCaption{}, ErrInvalid
		}
		rule = style
	}
	if in.Size != 0 {
		r := rule.Role()
		if float64(in.Size) < r.Min || float64(in.Size) > r.Size {
			return OwnerCaption{}, ErrInvalid
		}
	}
	out := OwnerCaption{Size: in.Size, Style: in.Style}
	if in.Position != nil {
		canvas, err := ClipCanvas(ratio)
		if err != nil {
			return OwnerCaption{}, err
		}
		at := ClampCaptionPlacement(canvas, *in.Position)
		out.Position = &at
	}
	return out, nil
}
