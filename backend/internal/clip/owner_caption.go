package clip

import (
	"math"
	"slices"

	"github.com/postpilot/backend/internal/clip/composition"
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
// rest where it is written (CDS-82, CLIP-143). Every product-approved style is
// the owner's to choose, inside the project's AI set or not: that set limits
// what a writer may pick, never the owner (CLIP-142, CDS-66), and it is read
// here only to know the style a caption naming none renders in. A style the
// product does not carry is an authoring error. A size outside the role of the
// style the caption renders in — below CDS-3's floor, or above the size the
// role is set at, which V2 refuses on the way out — is refused by name with
// the range that style admits, so the editor corrects it rather than having it
// reset (CDS-100). The POSITION is clamped rather than refused: CDS-82 clamps
// position only, and an owner who drags past the edge means the edge.
func ValidateOwnerCaption(in OwnerCaption, e composition.Element, ratio string, allowed []string) (OwnerCaption, error) {
	if e.Role != "caption" {
		if in != (OwnerCaption{}) {
			return OwnerCaption{}, ErrInvalid
		}
		return OwnerCaption{}, nil
	}
	if in.Style != "" {
		if _, ok := design.CaptionRule(in.Style); !ok {
			return OwnerCaption{}, ErrInvalid
		}
	}
	drawn, _ := CaptionStyleOf(PortableText{Owner: OwnerCaption{Style: in.Style}, Resolved: composition.ResolvedElement{Element: e}}, ResolvedCaptionStyles(allowed))
	rule, ok := design.CaptionRule(drawn)
	if !ok {
		rule = design.Caption()
	}
	if in.Size != 0 {
		r := rule.Role()
		if float64(in.Size) < r.Min || float64(in.Size) > r.Size {
			return OwnerCaption{}, &composition.Problem{ElementID: e.ID, Line: e.Span.Line, Reason: "caption_size",
				Min: int(math.Ceil(r.Min)), Max: int(r.Size), Actual: in.Size}
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

// CaptionStyleOf is the approved style one caption renders in (CLIP-142,
// CDS-66): the owner's own choice, then the style its plan names, then the
// project's first AI style for a caption that names none ("auto", written
// before every caption carried a style). The AI set limits what a writer may
// choose, never what an existing plan holds, so an approved style outside it is
// kept and changing the set restyles nothing (CLIP-191). A named style the
// product does not carry falls back to that first style, which the second
// result reports; an owner style is admitted where it is saved.
func CaptionStyleOf(text PortableText, allowed []string) (string, bool) {
	if text.Owner.Style != "" {
		return text.Owner.Style, false
	}
	style := text.Resolved.Element.Style
	if style == "" || style == "auto" {
		return allowed[0], false
	}
	if _, ok := design.CaptionRule(style); !ok {
		return allowed[0], true
	}
	return style, false
}

// RestrictGeneratedCaptionStyles holds a generated plan's own caption choices to
// the AI set the project allows now (CDS-66). A fresh narration was held to it
// when it was parsed; a plan kept from an earlier attempt was written against
// that attempt's set, and a set changed since must not let its old choices
// through. An owner's choice is not the writer's and is left alone (CLIP-142).
func RestrictGeneratedCaptionStyles(plan *EditPlan, allowed []string) {
	if plan.Portable == nil || len(allowed) == 0 {
		return
	}
	for i, text := range plan.Portable.Elements {
		e := text.Resolved.Element
		if e.Role != "caption" || text.Owner.Style != "" || e.Style == "" || e.Style == "auto" || slices.Contains(allowed, e.Style) {
			continue
		}
		plan.Portable.Elements[i].Resolved.Element.Style = allowed[0]
		AddPlanNotice(plan, "composition_caption_style", "", e.ID, "style_fallback")
	}
}
