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
	// CDS-9 (SA-C) and CDS-13.
	safe := map[string]design.Region{
		"vertical":   {X: 64, Y: 250, Width: 856, Height: 1170},
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
		"vertical":   {Top: 290, UpperMid: 700, LowerMid: 1100, Bottom: 1380, Left: 96, Center: 540, Right: 888},
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
	// CDS-9 records the Naver-only estimate beside SA-C; CDS-10 keeps the
	// overlay geometry a constant.
	if design.SafeNaverEstimate != (design.Region{X: 64, Y: 230, Width: 866, Height: 1210}) {
		t.Fatal("SA-N", design.SafeNaverEstimate)
	}
	if design.Overlay != (design.OverlayEstimate{Top: 230, Bottom: 1440, Right: 930, RightFromY: 1000, Left: 64}) {
		t.Fatal("overlay estimate", design.Overlay)
	}
}

func TestRatioLayoutsMatchCDS46To48(t *testing.T) {
	// CDS-46: every font size is kept except t.hook.
	want := map[string]design.RatioLayout{
		"vertical": {
			CopyMaxWidth: 856, HookSize: 84,
			HookCard: design.CardBox{Width: 792, CenterY: 840},
			EndCard:  design.CardBox{Width: 792, CenterY: 1040},
			Chip:     design.ChipStack{X: 96, Y: 290, MaxWidth: 600, Columns: 1},
			Badge:    design.BadgeBox{Right: 888, Top: 270},
			// CDS-32 states both 9:16 scrim rectangles exactly.
			ScrimTop:    design.Region{Y: 250, Width: 1080, Height: 310},
			ScrimBottom: design.Region{Y: 1040, Width: 1080, Height: 380},
		},
		"horizontal": {
			CopyMaxWidth: 960, HookSize: 72,
			HookCard:    design.CardBox{Width: 1120, CenterY: 540},
			EndCard:     design.CardBox{Width: 1120, CenterY: 540},
			Chip:        design.ChipStack{X: 96, Y: 112, MaxWidth: 600, Columns: 2},
			Badge:       design.BadgeBox{Right: 1824, Top: 92},
			ScrimTop:    design.Region{Y: 72, Width: 1920, Height: 200},
			ScrimBottom: design.Region{Y: 748, Width: 1920, Height: 260},
		},
		"square": {
			CopyMaxWidth: 952, HookSize: 76,
			HookCard:    design.CardBox{Width: 880, CenterY: 540},
			EndCard:     design.CardBox{Width: 880, CenterY: 560},
			Chip:        design.ChipStack{X: 64, Y: 112, MaxWidth: 600, Columns: 1},
			Badge:       design.BadgeBox{Right: 1016, Top: 92},
			ScrimTop:    design.Region{Y: 72, Width: 1080, Height: 200},
			ScrimBottom: design.Region{Y: 748, Width: 1080, Height: 260},
		},
	}
	for ratio, w := range want {
		got, ok := design.Layout(ratio)
		if !ok {
			t.Fatal(ratio)
		}
		got.Canvas, got.Safe, got.Anchor = design.Size{}, design.Region{}, design.Anchor{}
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
		"hook":    {Size: 84, Min: 72, Face: "paperlogy", Weight: 800, Tracking: -0.02, LineHeight: 1.15, Chars: 9},
		"title":   {Size: 72, Min: 64, Face: "pretendard", Weight: 800, Tracking: -0.02, LineHeight: 1.2, Chars: 11},
		"mark":    {Size: 60, Min: 52, Face: "pretendard", Weight: 800, Tracking: -0.01, LineHeight: 1.3},
		"body":    {Size: 56, Min: 48, Face: "pretendard", Weight: 700, Tracking: -0.01, LineHeight: 1.3, Chars: 14},
		"caption": {Size: 44, Min: 40, Face: "pretendard", Weight: 600, Tracking: 0, LineHeight: 1.3, Chars: 18},
		"label":   {Size: 36, Min: 34, Face: "pretendard", Weight: 600, Tracking: 0.02, LineHeight: 1.2},
		"badge":   {Size: 40, Min: 40, Face: "pretendard", Weight: 700, Tracking: 0.02, LineHeight: 1},
	}
	if !reflect.DeepEqual(design.Type, want) {
		t.Fatalf("type scale\n got %+v\nwant %+v", design.Type, want)
	}
	// CDS-3: no role may be typeset under the body floor of 48 px.
	if design.Type["body"].Min != 48 || design.Type["caption"].Min < 40 || design.Type["badge"].Size != 40 {
		t.Fatal("size floors")
	}
}

func TestColourAndSpacingTokensMatchCDS14And15And21(t *testing.T) {
	color := map[string]design.Paint{
		"ink_900":     {Hex: "#111111", Alpha: 0.72},
		"ink_900s":    {Hex: "#111111", Alpha: 0.88},
		"paper_50":    {Hex: "#FFFCF5", Alpha: 0.94},
		"text_white":  {Hex: "#FFFFFF", Alpha: 1},
		"text_ink":    {Hex: "#1A1A1A", Alpha: 1},
		"text_muted":  {Hex: "#FFFFFF", Alpha: 0.72},
		"stroke_dark": {Hex: "#0B0B0B", Alpha: 0.85},
		"badge_ad":    {Hex: "#111111", Alpha: 0.8},
	}
	if !reflect.DeepEqual(design.Color, color) {
		t.Fatalf("colour tokens %+v", design.Color)
	}
	shadow := map[string]design.ShadowPaint{
		"text": {Hex: "#000000", Alpha: 0.55, Blur: 12, DY: 4},
		"card": {Hex: "#000000", Alpha: 0.35, Blur: 32, DY: 8},
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
	spacing := design.SpacingTokens{
		PadBox: design.Pad{V: 22, H: 32}, PadChip: design.Pad{V: 12, H: 20},
		GapStack: 16, GapChip: 12,
		RadiusBox: 16, RadiusChip: 999, RadiusCard: 24,
		BarAccent: 8, StrokeText: 6,
		// 형광펜's 4 px stroke (CDS-26) and 메모's 14 px dot (CDS-24).
		StrokeMark: 4, DotAccent: 14,
		UnderlineMark: design.Underline{HeightEM: 0.42, RaiseEM: 0.28, Extend: 6},
	}
	if design.Spacing != spacing {
		t.Fatalf("spacing tokens %+v", design.Spacing)
	}
}

func TestStylesMotionTimingTransitionAudioAndLuma(t *testing.T) {
	// CDS-22 through CDS-26: four styles, one per role.
	styles := map[string]design.StyleRule{
		// CDS-23: plate, bar, pad.box with the bar inside a 40 px left inset.
		"clean": {Type: "body", Plate: "ink_900", Lines: 2, Chars: 14, Anchor: "bottom", Align: "center",
			Padding: design.Pad{V: 22, H: 32}, PadLeft: 40, Bar: true},
		// CDS-24: light plate, one accent dot, its own 18/28 padding.
		"memo": {Type: "caption", Plate: "paper_50", Lines: 1, Chars: 18, Anchor: "top", AnchorAlt: "bottom", Align: "left",
			Padding: design.Pad{V: 18, H: 28}, PadLeft: 54, Dot: true},
		// CDS-25 and CDS-26: no plate, a stroke under the fill and a drop shadow.
		"bold": {Type: "title", Lines: 2, Chars: 11, Anchor: "upper_mid", AnchorAlt: "lower_mid", Align: "center",
			Stroke: "text", Shadow: "text"},
		"mark": {Type: "mark", Lines: 1, Chars: 16, Anchor: "bottom", Align: "center",
			Stroke: "mark", Shadow: "text", Highlight: true},
	}
	if !reflect.DeepEqual(design.Styles, styles) {
		t.Fatalf("styles\n got %+v\nwant %+v", design.Styles, styles)
	}
	// 깔끔하게's padding IS pad.box (CDS-23), so the two have one value, not two.
	if design.Styles["clean"].Padding != design.Spacing.PadBox {
		t.Fatal("clean padding drifted from pad.box")
	}
	if design.Styles["bold"].StrokeWidth() != 6 || design.Styles["mark"].StrokeWidth() != 4 || design.Styles["clean"].StrokeWidth() != 0 {
		t.Fatal("stroke widths")
	}
	if design.Spacing.DotAccent != 14 || design.Spacing.StrokeMark != 4 {
		t.Fatal("style-specific spacing tokens", design.Spacing)
	}
	for id, s := range styles {
		if _, ok := design.Type[s.Type]; !ok {
			t.Fatalf("%s names an unknown type role %q", id, s.Type)
		}
		if s.Shadow != "" {
			if _, ok := design.Shadow[s.Shadow]; !ok {
				t.Fatalf("%s names an unknown shadow %q", id, s.Shadow)
			}
		}
		// An unplated style carries a stroke and a shadow; a plated one neither.
		if (s.Plate == "") != (s.Stroke != "" && s.Shadow != "") {
			t.Fatalf("%s mixes a plate with a stroke treatment", id)
		}
		if s.Plate != "" {
			if _, ok := design.Color[s.Plate]; !ok {
				t.Fatalf("%s names an unknown plate %q", id, s.Plate)
			}
		}
	}
	// CDS-4 and CDS-27.
	if design.Motion != (design.MotionTokens{InMS: 180, InDY: 12, OutMS: 120, Ease: [4]float64{0.22, 1, 0.36, 1}}) {
		t.Fatal("motion", design.Motion)
	}
	// CDS-41, CDS-37, CDS-28, CDS-29, CDS-5.
	if design.Timing != (design.TimingTokens{SubMinBaseMS: 900, SubMinPerCharMS: 90, CutMinS: 1.2, CutMaxS: 6, HookCardS: 1.5, EndCardS: 2.5, BadgeMinHeadS: 3, BadgeMinTailS: 3, ChipMinS: 2}) {
		t.Fatal("timing", design.Timing)
	}
	// CDS-36.
	if design.Transition != (design.TransitionTokens{Default: "cut", FadeMS: 200, FadeRatioMax: 0.4}) {
		t.Fatal("transition", design.Transition)
	}
	// CDS-35.
	if design.Audio != (design.AudioTokens{Loudnorm: design.Loudnorm{I: -16, TP: -1.5, LRA: 11}, CrossfadeMS: 60, HookDipDB: -6}) {
		t.Fatal("audio", design.Audio)
	}
	// CDS-44.
	if design.Luma != (design.LumaTokens{ScrimThreshold: 0.6, SigmaThreshold: 0.25}) {
		t.Fatal("luma", design.Luma)
	}
}

// CDS-7 promises identical output for identical input, which holds only while
// both sides read the same numbers. The frontend copy is the preview's and its
// validators' source, so byte equality is the only check that catches drift.
func TestFrontendMirrorIsByteIdentical(t *testing.T) {
	mirror, err := os.ReadFile("../../../../frontend/src/shared/config/clip-design.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(mirror) != string(design.JSON()) {
		t.Fatal("frontend/src/shared/config/clip-design.json drifted from the embedded design.json")
	}
}
