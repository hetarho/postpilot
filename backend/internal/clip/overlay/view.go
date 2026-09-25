package overlay

// These versioned views contain resolved geometry and plain text only. Layout
// policy and glyph measurement stay with the caller; SVG assets own drawing.
type Canvas struct{ Width, Height int }
type Box struct {
	X, Y, Width, Height, Radius float64
	Fill, Opacity               string
}
type Circle struct {
	X, Y, Radius float64
	Fill         string
}
type Shadow struct {
	DX, DY, Deviation float64
	Fill, Opacity     string
}
type Scrim struct {
	Box
	From, To string
}
type Text struct {
	X, Y, Size, Tracking, StrokeWidth, Length    float64
	Weight                                       int
	Family, Fill, Opacity, Stroke, StrokeOpacity string
	Value, Prefix, Keyword, Suffix, Accent       string
	Colored, Shadow                              bool
	Highlight                                    *Box
}
type CopyView struct {
	Canvas
	Scrim      *Scrim
	Shadow     *Shadow
	Plate, Bar *Box
	Dot        *Circle
	Lines      []Text
}
type Chip struct {
	Box
	Label, Value Text
}
type FurnitureView struct {
	Canvas
	Shadow *Shadow
	Badge  *Box
	Label  *Text
	Chips  []Chip
}

// RegionView is region-v2: region-v1's rules and lines plus a preset's neutral
// decoration, lines set along an arc, a radial scrim and one rotated group.
type RegionView struct {
	CopyView
	Rules  []Box
	Radial *Radial
	Shapes []Shape
	Arcs   []ArcText
	Turn   *Turn
}

// Shape is a rectangle, rounded by Radius, or with Circle a circle of Radius
// centred on X, Y.
type Shape struct {
	Box
	Circle                bool
	Stroke, StrokeOpacity string
	StrokeWidth           float64
	Shadow                bool
}

// ArcText is one line centred along the path its own ID names.
type ArcText struct {
	Text
	ID, Path string
}

// Radial is an elliptical scrim: opaque From at its centre through Mid at MidAt
// to To at its edge.
type Radial struct {
	CX, CY, RX, RY             float64
	Fill, From, Mid, MidAt, To string
}

// Turn holds the parts drawn rotated by Deg about X, Y.
type Turn struct {
	Deg, X, Y float64
	Rules     []Box
	Shapes    []Shape
	Lines     []Text
	Arcs      []ArcText
}
