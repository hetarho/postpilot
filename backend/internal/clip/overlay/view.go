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
	Badge *Box
	Label *Text
	Chips []Chip
}
type CardLine struct {
	Chip *Box
	Text Text
}
type CardView struct {
	Canvas
	Empty  bool
	Shadow Shadow
	Plate  Box
	Lines  []CardLine
}
