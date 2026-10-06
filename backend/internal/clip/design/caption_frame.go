package design

import (
	"bytes"
	"encoding/xml"
	"math"
	"strconv"
	"strings"
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

// A sampled unplated caption owns this backdrop alongside its own ink.
type CaptionScrim struct {
	Region Bounds
	Paint  ScrimPaint
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
	GroundScrim *CaptionScrim
	Canvas      Size
	Style       CaptionStyle
	Family      string
	Size        float64
	Tracking    float64
	Lines       []CaptionLine
	Region      Bounds
	// The project's accent, already resolved to a hex colour, or empty where
	// the project chose none or the style admits none (CDS-15).
	Accent string
	// Progress runs 0 → 1 across the caption's own interval, whose length in
	// milliseconds is what turns the style's declared motion into this frame's
	// opacity and offset.
	Progress   float64
	DurationMS int
	// The caption's characters its style's face does not draw, set in Wanted
	// Sans Variable inside the style's own drawing (CDS-84).
	Substitute map[rune]bool
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

// CaptionMarkup is a caption string as the content of its <text>: escaped, with
// each run of characters its style's face does not draw set in Wanted Sans
// Variable (CDS-84). The run inherits the element's weight, size, tracking and
// paint, so only the letterform changes. With nothing to substitute it is the
// escaped text alone, byte for byte what a caption always carried.
func CaptionMarkup(text string, substitute map[rune]bool) string {
	var b strings.Builder
	for _, run := range SplitSubstituted(text, substitute) {
		if run.Substituted {
			b.WriteString(`<tspan font-family="` + FontFamily("wantedsans") + `">` + esc(run.Text) + `</tspan>`)
		} else {
			b.WriteString(esc(run.Text))
		}
	}
	return b.String()
}

// A stretch of a caption string that is, or is not, set in the substitute.
type SubstitutedRun struct {
	Text        string
	Substituted bool
}

// SplitSubstituted cuts text into maximal runs by whether each character is in
// the substitute set. An empty set yields the whole text as one plain run.
func SplitSubstituted(text string, substitute map[rune]bool) []SubstitutedRun {
	if len(substitute) == 0 {
		if text == "" {
			return nil
		}
		return []SubstitutedRun{{Text: text}}
	}
	var out []SubstitutedRun
	start, current := 0, false
	for i, c := range text {
		sub := substitute[c]
		if i > start && sub != current {
			out = append(out, SubstitutedRun{Text: text[start:i], Substituted: current})
			start = i
		}
		current = sub
	}
	if start < len(text) {
		out = append(out, SubstitutedRun{Text: text[start:], Substituted: current})
	}
	return out
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
