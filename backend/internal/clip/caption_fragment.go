package clip

import "context"

// CaptionFragment is one caption exactly as the RENDERER draws it, moved to the
// origin (CDS-83). The editor places it with one transform, so dragging a
// caption never asks the server anything, and it cannot draw something the
// render will not produce: the fragment comes from the same style registry, the
// same measured lines and the same painter the render uses.
type CaptionFragment struct {
	InstanceID, Style string
	// A `<g>` holding the caption drawn at the origin, with the `<defs>` its
	// filters need. Every id inside is prefixed with this caption's own
	// instance, because two captions on screen would otherwise collide on a
	// filter id and the browser would paint one of them with the other's.
	SVG string
	// Where the caption's measured bounds sit on the canvas today, which is
	// what the caller translates the fragment by.
	Box      Region
	FontSize float64
	// A sequence-rendered style draws one layer per output frame (CDS-81); this
	// is ONE representative frame of that motion, not the motion itself.
	Sequence bool
}

// CaptionPreview is what ② needs to draw every caption of a plan: the fragments,
// the canvas they are placed on and the safe area they are clamped to, so the
// editor mirrors no CDS-9/CDS-13 constant of its own.
type CaptionPreview struct {
	Ratio     string
	Canvas    Canvas
	Fragments []CaptionFragment
}

type CaptionFragmenter interface {
	CaptionFragments(context.Context, EditPlan, []RenderSource) ([]CaptionFragment, error)
}
