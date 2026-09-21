package design

import (
	"fmt"
	"slices"
)

// The approved caption style set (CDS-80). The registry is Go rather than one
// more `design.json` map because a style carries a filter graph and generated
// geometry a constants file cannot express; the numbers the renderer and the
// preview must agree on to the pixel stay in `regions.caption`, which both
// sides read.
//
// Every style fixes four things: the face it is set in, the type role it takes
// its scale and per-line limits from, the colour treatment it paints, and the
// motion it declares. A style is also either static — one rasterisation
// repeated for its whole interval — or sequence-rendered, one layer per output
// frame (CDS-81), because the two cost differently enough to be quoted apart.
const (
	StaticCaption   = "static"
	SequenceCaption = "sequence"
)

// CaptionPaint is a style's colour treatment. Stroke names the stroke's colour
// only: its width is the rule's CDS-21 token, so no style invents a width.
// An empty field means the style paints nothing there.
type CaptionPaint struct {
	Fill   string
	Stroke string
	Shadow ShadowPaint
	// The shape a style draws its own text on — a sticker, a speech bubble, a
	// stacked block. Empty for every unplated style, which is most of them.
	Plate string
	// Accent admits CDS-15's one accent word on a dark ground. A style that
	// paints its own colour across the whole line does not, because a second
	// colour inside it reads as a mistake rather than as an accent.
	Accent bool
	// Scrim admits CDS-44's scrim under this style when the ground is bright.
	Scrim bool
}

// CaptionStyle is one member of the approved set.
type CaptionStyle struct {
	ID string
	// The Korean name CDS and the owner's own surface call it by.
	Name string
	// What this treatment reads as, supplied with the allowed narration styles.
	Description string
	// A key in Faces, which resolves to the CSS family the bundled file
	// declares. The style's face outranks the type role's (CDS-18).
	Face   string
	Weight int
	// Tracking in em, overriding the role's where the style asks for one.
	Tracking float64
	// StaticCaption or SequenceCaption (CDS-80, CDS-81).
	Rendering string
	Motion    MotionTokens
	Paint     CaptionPaint
	rule      StyleRule
}

// Static answers the question the quote and the render budget both ask.
func (s CaptionStyle) Static() bool { return s.Rendering == StaticCaption }

// Role is the style's own scale entry (CDS-19, CDS-20), with the face, weight
// and tracking the style names rather than the ones the role carries for its
// region use (CDS-18).
func (s CaptionStyle) Role() TypeRole {
	role := Type[s.rule.Type]
	role.Face, role.Weight = s.Face, s.Weight
	if s.Tracking != 0 {
		role.Tracking = s.Tracking
	}
	return role
}

// PerWord reports whether this style treats the words of a line one at a time,
// which is what makes their measured boxes part of the layout rather than a
// drawing detail.
func (s CaptionStyle) PerWord() bool { return s.ID == "word-pop" || s.ID == "pop" }

// DarkStroke reports whether this style's stroke is the `stroke.dark` CDS-44
// measures stroked contrast against. A style stroking in any other colour is
// measured against its own ground instead, because a coloured outline is
// decoration rather than a backing the text can be read off.
func (s CaptionStyle) DarkStroke() bool { return s.Paint.Stroke == Color["stroke_dark"].Hex }

// Rule is the layout contract the fit loop and the manifest read: the anchors,
// the line and character limits, the stroke token and the insets.
func (s CaptionStyle) Rule() StyleRule { return s.rule }

// DefaultCaptionStyle is the treatment an empty selection resolves to, and the
// one a caption falls back to when its own face cannot set a syllable (CDS-25,
// CDS-84).
const DefaultCaptionStyle = "bold"

var captionStyles = buildCaptionStyles()

// The set, in the order a surface offers it: the default first, then the
// remaining static styles, then the sequence-rendered ones.
func buildCaptionStyles() []CaptionStyle {
	white := Color["text_white"].Hex
	set := []CaptionStyle{{
		ID: DefaultCaptionStyle, Description: "Large, bold emphasis with a dark outline.", Name: "크게 강조", Face: "paperlogy", Weight: 800, Rendering: StaticCaption, Motion: Motion,
		Paint: CaptionPaint{Fill: white, Stroke: Color["stroke_dark"].Hex, Shadow: Shadow["text"], Accent: true, Scrim: true},
	}, {
		// 400 is the LIGHTEST weight this face has: Wanted Sans Variable's wght axis runs
		// 400–1000, where the face it replaced ran 250–900 (owner decision 2026-09-21). A style
		// asking for 250 would not fail — the renderer would silently clamp it — so the number
		// states the weight the render actually carries.
		ID: "keynote", Description: "Quiet, light sans-serif presentation.", Name: "키노트", Face: "wantedsans", Weight: 400, Tracking: -0.026, Rendering: StaticCaption,
		Motion: MotionTokens{InMS: 320, InDY: 10, OutMS: 200, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: white, Shadow: ShadowPaint{Hex: "#000000", Alpha: 0.5, Blur: 28, DY: 2}, Scrim: true},
	}, {
		ID: "film", Description: "Cinematic serif subtitles.", Name: "필름 자막", Face: "nanummyeongjo", Weight: 800, Tracking: 0.007, Rendering: StaticCaption,
		Motion: MotionTokens{InMS: 200, InDY: 0, OutMS: 280, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: "#F2F2EE", Shadow: ShadowPaint{Hex: "#000000", Alpha: 0.92, Blur: 10, DY: 2}},
	}, {
		ID: "word-pop", Description: "Words highlighted one at a time for rhythmic emphasis.", Name: "워드 팝", Face: "wantedsans", Weight: 800, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 80, InDY: 0, OutMS: 120, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: white, Stroke: "#0A0C10", Accent: true},
	}, {
		ID: "blur-in", Description: "Text comes into focus from a soft blur.", Name: "블러 인", Face: "wantedsans", Weight: 700, Tracking: -0.016, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 360, InDY: 0, OutMS: 200, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: white, Shadow: Shadow["text"], Scrim: true},
	}, {
		ID: "ambient", Description: "Soft light drifting around the text.", Name: "소프트 앰비언트", Face: "wantedsans", Weight: 700, Tracking: -0.011, Rendering: SequenceCaption,
		Motion: Motion,
		Paint:  CaptionPaint{Fill: white, Shadow: ShadowPaint{Hex: "#0A0C10", Alpha: 0.55, Blur: 14, DY: 0}},
	}, {
		ID: "neon", Description: "A bright cyan neon glow.", Name: "네온 사인", Face: "wantedsans", Weight: 800, Tracking: -0.01, Rendering: SequenceCaption,
		Motion: Motion,
		Paint:  CaptionPaint{Fill: "#EAFEFF"},
	}, {
		ID: "iridescent", Description: "Shifting rainbow light over bold text.", Name: "이리데센트", Face: "paperlogy", Weight: 800, Rendering: SequenceCaption,
		Motion: Motion,
		Paint:  CaptionPaint{Fill: white, Stroke: white},
	}, {
		ID: "glitch", Description: "Brief digital distortion for sharp emphasis.", Name: "글리치", Face: "paperlogy", Weight: 800, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 80, InDY: 0, OutMS: 120, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: white},
	}, {
		ID: "ember", Description: "A warm ember glow around the letters.", Name: "엠버 글로우", Face: "paperlogy", Weight: 800, Rendering: SequenceCaption,
		Motion: Motion,
		Paint:  CaptionPaint{Fill: "#FFF8EC", Stroke: "#7A2400"},
	}, {
		ID: "stack", Description: "Text stacked on dark blocks.", Name: "스택 블록", Face: "paperlogy", Weight: 800, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 340, InDY: 0, OutMS: 120, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: white, Plate: "#0A0C10", Accent: true},
	}, {
		ID: "outline", Description: "Hollow outlined letters with animated emphasis.", Name: "아웃라인", Face: "paperlogy", Weight: 800, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 180, InDY: 8, OutMS: 120, Ease: Motion.Ease},
		Paint:  CaptionPaint{Stroke: white, Accent: true},
	}, {
		ID: "pop", Description: "Rounded playful letters that bounce.", Name: "팝 바운스", Face: "jua", Weight: 400, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 300, InDY: 0, OutMS: 120, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: white, Stroke: "#FF3B6B"},
	}, {
		ID: "sticker", Description: "Rounded text on a white sticker.", Name: "스티커", Face: "jua", Weight: 400, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 260, InDY: 0, OutMS: 120, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: "#1A1C22", Plate: "#FFFFFF"},
	}, {
		ID: "bubble", Description: "Conversational text in a speech bubble.", Name: "버블 챗", Face: "jua", Weight: 400, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 280, InDY: 34, OutMS: 120, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: "#10130A", Plate: "#FFFFFF"},
	}, {
		ID: "serif", Description: "Spaced, restrained serif lettering with a gentle entrance.", Name: "세리프 미니멀", Face: "nanummyeongjo", Weight: 400, Tracking: 0.06, Rendering: SequenceCaption,
		Motion: MotionTokens{InMS: 300, InDY: 16, OutMS: 200, Ease: Motion.Ease},
		Paint:  CaptionPaint{Fill: "#F7F3EA", Shadow: ShadowPaint{Hex: "#000000", Alpha: 0.72, Blur: 18, DY: 2}},
	}}
	// A style with no `regions.caption` entry, or an entry with no style, is a
	// build mistake the process must not start with, exactly like a malformed
	// design.json: the preview draws from the entry and the render from the
	// style, and CDS-83 refuses a set where the two can disagree.
	for i := range set {
		rule, ok := loaded.Regions.Caption[set[i].ID]
		if !ok {
			panic(fmt.Errorf("clip caption style %q has no regions.caption entry", set[i].ID))
		}
		if _, ok := Faces[set[i].Face]; !ok {
			panic(fmt.Errorf("clip caption style %q names the unbundled face %q", set[i].ID, set[i].Face))
		}
		set[i].rule = rule
	}
	if len(set) != len(loaded.Regions.Caption) {
		panic(fmt.Errorf("clip caption styles: %d registered against %d regions.caption entries", len(set), len(loaded.Regions.Caption)))
	}
	return set
}

// CaptionStyles is the whole approved set, in offer order.
func CaptionStyles() []CaptionStyle { return slices.Clone(captionStyles) }

// LookupCaptionStyle finds one approved style by id. A name the set does not
// carry is refused rather than resolved to the default (CDS-66, CDS-80).
func LookupCaptionStyle(id string) (CaptionStyle, bool) {
	if i := slices.IndexFunc(captionStyles, func(s CaptionStyle) bool { return s.ID == id }); i >= 0 {
		return captionStyles[i], true
	}
	return CaptionStyle{}, false
}

// CaptionRule is the layout contract for one approved style.
func CaptionRule(id string) (StyleRule, bool) {
	style, ok := LookupCaptionStyle(id)
	return style.rule, ok
}

// Caption is the default caption treatment (CDS-25).
func Caption() StyleRule { return DefaultCaption().rule }

// DefaultCaption is the whole default style — what an empty selection resolves
// to, and what a caption whose own face lacks a syllable falls back to (CDS-84).
func DefaultCaption() CaptionStyle {
	style, _ := LookupCaptionStyle(DefaultCaptionStyle)
	return style
}
