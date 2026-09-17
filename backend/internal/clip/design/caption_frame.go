package design

import (
	"bytes"
	"encoding/xml"
	"math"
	"strconv"
)

// What a caption style is handed to draw one layer. Everything here is already
// measured and placed in canvas coordinates: a style decides colour, filter and
// motion, never where the text sits or how large it is, because the layout, the
// verifier and the preview all read that from the manifest (CDS-83).
type CaptionWord struct {
	Text  string
	X     float64 // left edge of this word's ink, on the canvas
	Width float64
}

// One laid-out line: the <text> element's own x and baseline y, plus the ink box
// the measurement produced, all on the canvas.
type CaptionLine struct {
	Text          string
	X, Y          float64
	Top           float64
	Width, Height float64
	Words         []CaptionWord
	// The accent word on this line, when the copy named one this line holds.
	Keyword      string
	KeywordX     float64
	KeywordWidth float64
}

// CaptionFrame is one caption at one instant of its own interval.
type CaptionFrame struct {
	Canvas   Size
	Style    CaptionStyle
	Family   string
	Size     float64
	Tracking float64
	Lines    []CaptionLine
	Region   Bounds
	// The project's accent, already resolved to a hex colour, or empty where
	// the project chose none or the style admits none (CDS-15).
	Accent string
	// Progress runs 0 → 1 across the caption's own interval, whose length in
	// milliseconds is what turns the style's declared motion into this frame's
	// opacity and offset.
	Progress   float64
	DurationMS int
}

// Bleed is how far outside the caption's own box this style paints, so the
// layer it draws can be cropped to the pixels it actually touches. A style that
// only strokes and shadows its text stays inside the shared minimum.
func (s CaptionStyle) Bleed() float64 {
	switch s.ID {
	case "ember":
		// Its tongues rise from inside the letters and its sparks carry on past
		// them, which is the furthest anything in the set reaches.
		return 420
	case "ambient", "neon":
		return 180
	case "sticker", "bubble", "serif", "stack":
		// Each draws a shape or a rule around the text and casts a shadow off it.
		return 80
	case "glitch", "iridescent":
		return 60
	case "pop":
		return 48
	}
	return 40
}

func esc(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

// num formats a coordinate the same way everywhere, so two runs of one frame are
// byte-identical and a golden means something.
func num(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func easeOutCubic(t float64) float64 { return 1 - math.Pow(1-t, 3) }

func easeOutBack(t float64) float64 {
	const s = 1.70158
	return 1 + (s+1)*math.Pow(t-1, 3) + s*math.Pow(t-1, 2)
}

// entrance is the opacity and the upward offset a style's declared motion puts
// on this frame: the same in/out milliseconds and the same settle distance the
// manifest carries and V9 checks, expressed as a fraction of the interval.
func (f CaptionFrame) entrance() (opacity, dy float64) {
	m := f.Style.Motion
	if f.DurationMS <= 0 {
		return 1, 0
	}
	elapsed := f.Progress * float64(f.DurationMS)
	in := clamp01(elapsed / math.Max(1, float64(m.InMS)))
	out := clamp01((float64(f.DurationMS) - elapsed) / math.Max(1, float64(m.OutMS)))
	return math.Min(easeOutCubic(in), out), m.InDY * (1 - easeOutCubic(in))
}

// settled is how far a style's own entrance has run at this frame, 0 → 1 over
// its declared in-milliseconds.
func (f CaptionFrame) settled() float64 {
	if f.DurationMS <= 0 || f.Style.Motion.InMS <= 0 {
		return 1
	}
	return clamp01(f.Progress * float64(f.DurationMS) / float64(f.Style.Motion.InMS))
}
