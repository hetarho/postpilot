package design_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// Every number below is quoted from a CDS decision. The test exists so a change
// to the shared configuration file cannot silently move the design system.
func TestSafeAreasAndAnchorsMatchCDS(t *testing.T) {
	// CDS-9 (design bounds) and CDS-13.
	safe := map[string]design.Bounds{
		"vertical":   {X: 64, Y: 40, Width: 952, Height: 1380},
		"horizontal": {X: 96, Y: 72, Width: 1728, Height: 936},
		"square":     {X: 64, Y: 72, Width: 952, Height: 936},
	}
	canvas := map[string]design.Size{
		"vertical":   {Width: 1080, Height: 1920},
		"horizontal": {Width: 1920, Height: 1080},
		"square":     {Width: 1080, Height: 1080},
	}
	// CDS-12 for 9:16; CDS-47 and CDS-48 restate only what the other two change.
	anchors := map[string]design.Anchor{
		"vertical":   {Top: 80, UpperMid: 700, LowerMid: 1100, Bottom: 1380, Left: 96, Center: 540, Right: 984},
		"horizontal": {Top: 112, UpperMid: 420, LowerMid: 660, Bottom: 968, Left: 96, Center: 960, Right: 1824},
		"square":     {Top: 112, UpperMid: 420, LowerMid: 660, Bottom: 968, Left: 64, Center: 540, Right: 1016},
	}
	if len(design.Ratios) != 3 {
		t.Fatal("ratio count", len(design.Ratios))
	}
	for ratio, want := range safe {
		got, ok := design.Safe(ratio)
		if !ok || got != want {
			t.Fatalf("%s safe area %+v", ratio, got)
		}
		if c, _ := design.Canvas(ratio); c != canvas[ratio] {
			t.Fatalf("%s canvas %+v", ratio, c)
		}
		if a, _ := design.Anchors(ratio); a != anchors[ratio] {
			t.Fatalf("%s anchors %+v", ratio, a)
		}
		// CDS-2: no anchor may place a zero-size element outside the safe area.
		a := anchors[ratio]
		for _, y := range []float64{a.Top, a.UpperMid, a.LowerMid, a.Bottom} {
			if y < want.Y || y > want.Y+want.Height {
				t.Fatalf("%s vertical anchor %v outside safe area", ratio, y)
			}
		}
		for _, x := range []float64{a.Left, a.Center, a.Right} {
			if x < want.X || x > want.X+want.Width {
				t.Fatalf("%s horizontal anchor %v outside safe area", ratio, x)
			}
		}
	}
	// CDS-78: every ratio centres on its own canvas, with no platform inset.
	for ratio, layout := range design.Ratios {
		if layout.Anchor.Center != float64(layout.Canvas.Width)/2 {
			t.Fatalf("%s centre %v is not the canvas centre %v", ratio, layout.Anchor.Center, float64(layout.Canvas.Width)/2)
		}
		if layout.Safe.X != float64(layout.Canvas.Width)-(layout.Safe.X+layout.Safe.Width) {
			t.Fatalf("%s safe area insets %v and %v are not equal", ratio, layout.Safe.X, float64(layout.Canvas.Width)-(layout.Safe.X+layout.Safe.Width))
		}
		if layout.Anchor.Left != float64(layout.Canvas.Width)-layout.Anchor.Right {
			t.Fatalf("%s LEFT %v and RIGHT %v are not symmetric", ratio, layout.Anchor.Left, layout.Anchor.Right)
		}
	}
}

func TestRatioLayoutsMatchCDS46To48(t *testing.T) {
	// CDS-46: every font size is kept except t.hook.
	want := map[string]design.RatioLayout{
		"vertical": {
			CopyMaxWidth: 856, HookSize: 84,
			Badge: design.BadgeBox{Right: 984, Top: 80},
			// CDS-32 states both 9:16 scrim rectangles exactly.
			ScrimTop:    design.Bounds{Y: 40, Width: 1080, Height: 310},
			ScrimBottom: design.Bounds{Y: 1040, Width: 1080, Height: 380},
		},
		"horizontal": {
			CopyMaxWidth: 960, HookSize: 72,
			Badge:       design.BadgeBox{Right: 1824, Top: 112},
			ScrimTop:    design.Bounds{Y: 72, Width: 1920, Height: 200},
			ScrimBottom: design.Bounds{Y: 748, Width: 1920, Height: 260},
		},
		"square": {
			CopyMaxWidth: 952, HookSize: 76,
			Badge:       design.BadgeBox{Right: 1016, Top: 112},
			ScrimTop:    design.Bounds{Y: 72, Width: 1080, Height: 200},
			ScrimBottom: design.Bounds{Y: 748, Width: 1080, Height: 260},
		},
	}
	for ratio, w := range want {
		got, ok := design.Layout(ratio)
		if !ok {
			t.Fatal(ratio)
		}
		got.Canvas, got.Safe, got.Anchor = design.Size{}, design.Bounds{}, design.Anchor{}
		if got != w {
			t.Fatalf("%s layout\n got %+v\nwant %+v", ratio, got, w)
		}
		// A scrim shares the safe area's own top and bottom edge (CDS-32).
		safe, _ := design.Safe(ratio)
		if w.ScrimTop.Y != safe.Y || w.ScrimBottom.Y+w.ScrimBottom.Height != safe.Y+safe.Height {
			t.Fatalf("%s scrims leave the safe area", ratio)
		}
	}
}

func TestTypeScaleAndCharacterCountsMatchCDS19And20(t *testing.T) {
	want := map[string]design.TypeRole{
		"display":  {Size: 132, Min: 132, Face: "paperlogy", Weight: 800, Tracking: 0.04, LineHeight: 1, Chars: 6, Floor: 80},
		"headline": {Size: 96, Min: 96, Face: "paperlogy", Weight: 800, Tracking: -0.02, LineHeight: 1.1, Chars: 8, Floor: 60},
		"hook":     {Size: 84, Min: 72, Face: "paperlogy", Weight: 800, Tracking: -0.02, LineHeight: 1.15, Chars: 9, Floor: 56},
		// CDS-17 gives the hook title and 크게 강조 to the secondary face, which
		// CDS-25's parenthetical made conditional on it being bundled. It is now.
		"title":   {Size: 72, Min: 64, Face: "paperlogy", Weight: 800, Tracking: -0.02, LineHeight: 1.2, Chars: 11, Floor: 52},
		"body":    {Size: 56, Min: 48, Face: "wantedsans", Weight: 700, Tracking: -0.01, LineHeight: 1.3, Chars: 14, Floor: 48},
		"caption": {Size: 44, Min: 40, Face: "wantedsans", Weight: 600, Tracking: 0, LineHeight: 1.3, Chars: 18, Floor: 40},
		"label":   {Size: 36, Min: 34, Face: "wantedsans", Weight: 600, Tracking: 0.02, LineHeight: 1.2, Chars: 22, Floor: 34},
		"badge":   {Size: 36, Min: 36, Face: "wantedsans", Weight: 800, Tracking: 0.02, LineHeight: 1, Floor: 36},
	}
	if !reflect.DeepEqual(design.Type, want) {
		t.Fatalf("type scale\n got %+v\nwant %+v", design.Type, want)
	}
	// Exactly the four faces CDS-17 names, each as the bundled file itself
	// declares it: a name SVG cannot parse as a CSS family silently falls back.
	if !reflect.DeepEqual(design.Faces, map[string]string{
		"wantedsans": "Wanted Sans Variable", "paperlogy": "Paperlogy",
		"jua": "Jua", "nanummyeongjo": "NanumMyeongjo",
	}) {
		t.Fatalf("faces %+v", design.Faces)
	}
	for role, entry := range design.Type {
		if design.FontFamily(entry.Face) == "" {
			t.Fatalf("%s names an unbundled face %q", role, entry.Face)
		}
	}
	// CDS-3: no role may be typeset under the body floor of 48 px.
	if design.Type["body"].Min != 48 || design.Type["caption"].Min < 40 || design.Type["badge"].Size != 36 {
		t.Fatal("size floors")
	}
}

func TestColourAndSpacingTokensMatchCDS14And15And21(t *testing.T) {
	color := map[string]design.Paint{
		"text_white":  {Hex: "#FFFFFF", Alpha: 1},
		"text_muted":  {Hex: "#FFFFFF", Alpha: 0.72},
		"stroke_dark": {Hex: "#0B0B0B", Alpha: 0.85},
		"badge_ad":    {Hex: "#111111", Alpha: 0.8},
	}
	if !reflect.DeepEqual(design.Color, color) {
		t.Fatalf("colour tokens %+v", design.Color)
	}
	shadow := map[string]design.ShadowPaint{
		"text": {Hex: "#000000", Alpha: 0.55, Blur: 12, DY: 4},
	}
	if !reflect.DeepEqual(design.Shadow, shadow) {
		t.Fatalf("shadows %+v", design.Shadow)
	}
	scrim := map[string]design.ScrimPaint{
		"top":    {Hex: "#000000", From: 0.45, To: 0},
		"bottom": {Hex: "#000000", From: 0, To: 0.55},
	}
	if !reflect.DeepEqual(design.Scrim, scrim) {
		t.Fatalf("scrims %+v", design.Scrim)
	}
	// CDS-15: exactly the seven approved accents, one per project.
	accent := map[string]string{
		"coral": "#FF6B57", "amber": "#FFB020", "lime": "#9BD53A", "teal": "#2BB8A6",
		"blue": "#3D7BFF", "violet": "#8A63FF", "pink": "#FF6FB5",
	}
	if !reflect.DeepEqual(design.Accent, accent) {
		t.Fatalf("accent palette %+v", design.Accent)
	}
	spacing := design.SpacingTokens{PadChip: design.Pad{V: 16, H: 28}, GapStack: 16, RadiusChip: 12, StrokeText: 6, StrokeSmall: 4}
	if design.Spacing != spacing {
		t.Fatalf("spacing tokens %+v", design.Spacing)
	}
}

func TestStylesMotionTimingTransitionAudioAndLuma(t *testing.T) {
	want := design.StyleRule{Type: "title", Lines: 2, Chars: 11, Anchor: "upper_mid", AnchorAlt: "lower_mid", Align: "center", Stroke: "text", Shadow: "text"}
	if design.Caption() != want || design.Caption().StrokeWidth() != 6 {
		t.Fatal("caption", design.Caption())
	}
	// CDS-4 and CDS-27.
	if design.Motion != (design.MotionTokens{InMS: 180, InDY: 12, OutMS: 120, Ease: [4]float64{0.22, 1, 0.36, 1}}) {
		t.Fatal("motion", design.Motion)
	}
	// CDS-41, CDS-37, CDS-28, CDS-29, CDS-5.
	if design.Timing != (design.TimingTokens{SubMinBaseMS: 900, SubMinPerCharMS: 90, CutMinS: 1.2, CutMaxS: 6, CutMaxFoodS: 4, IntroDefaultS: 2.5, OutroDefaultS: 3, BadgeMinHeadS: 3, BadgeMinTailS: 3, ChipMinS: 2, CopyLeadMS: 120, SubExtendMS: 240, SubOccupancyMin: 0.6}) {
		t.Fatal("timing", design.Timing)
	}
	// CDS-36.
	if design.Transition != (design.TransitionTokens{Default: "cut", FadeMS: 200, BlackMS: 300, FadeRatioMax: 0.4}) {
		t.Fatal("transition", design.Transition)
	}
	// CDS-35.
	if design.Audio != (design.AudioTokens{Loudnorm: design.Loudnorm{I: -16, TP: -1.5, LRA: 11}, CrossfadeMS: 60, HookDipDB: -6}) {
		t.Fatal("audio", design.Audio)
	}
	// CDS-44 and CDS-16's 4.5:1 floor, which V3 gates every pairing on.
	if design.Luma != (design.LumaTokens{ScrimThreshold: 0.6, SigmaThreshold: 0.25, ContrastMin: 4.5}) {
		t.Fatal("luma", design.Luma)
	}
}

// CDS-7 promises identical output for identical input, which holds only while
// both sides read the same numbers. The frontend copy is the preview's and its
// validators' source, so byte equality is the only check that catches drift.
func TestFrontendMirrorIsByteIdentical(t *testing.T) {
	mirror, err := os.ReadFile("../../../../frontend/src/entities/clip-design/config/clip-design.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(mirror) != string(design.JSON()) {
		t.Fatal("frontend/src/entities/clip-design/config/clip-design.json drifted from the embedded design.json")
	}
}
