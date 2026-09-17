package media

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// bundledFontPaths is the repository's own copy of every face the image ships.
func bundledFontPaths(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{
		"pretendard":        "pretendard/PretendardVariable.ttf",
		"paperlogy":         "paperlogy/Paperlogy-8ExtraBold.ttf",
		"jua":               "jua/Jua-Regular.ttf",
		"nanummyeongjo":     "nanummyeongjo/NanumMyeongjo-Regular.ttf",
		"nanummyeongjo-800": "nanummyeongjo/NanumMyeongjo-ExtraBold.ttf",
	}
	out := map[string]string{}
	for key, name := range files {
		path, err := filepath.Abs("../../../assets/fonts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		out[key] = path
	}
	return out
}

func testRenderer(t *testing.T, a *Adapter) *Rendering {
	t.Helper()
	cfg := renderConfig(t)
	cfg.FontPaths = bundledFontPaths(t)
	r, err := NewRenderer(a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestBundledFontAndGraphemeBoundaries(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	for _, text := range []string{"정확한 한글 & 여행", `<hello> "world"`} {
		if err := r.checkCopy(text, design.Type["body"]); err != nil {
			t.Fatalf("%q: %v", text, err)
		}
	}
	for _, text := range []string{"\x00", "\r", "🙂"} {
		if err := r.checkCopy(text, design.Type["body"]); err == nil {
			t.Fatalf("unsupported text %q", text)
		}
	}
	candidates, _, err := copyCandidates("a\u0308한글", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range candidates {
		if strings.Join(c, "") != "a\u0308한글" {
			t.Fatal("text was changed")
		}
		if len(c) == 2 && c[0] == "a" {
			t.Fatal("split a grapheme")
		}
	}
	if _, _, err := copyCandidates("one\ntwo\nthree", 2); !errors.Is(err, clip.ErrCopyTooLong) {
		t.Fatal(err)
	}
	if _, values, err := copyCandidates("same\nsame", 2); err != nil || len(values) != 1 {
		t.Fatalf("duplicate line measurement: %v %v", values, err)
	}
	// 메모 and 형광펜 are exactly one line (CDS-24, CDS-26): a second one is
	// refused outright rather than wrapped away.
	if _, _, err := copyCandidates("one\ntwo", 1); !errors.Is(err, clip.ErrCopyTooLong) {
		t.Fatal(err)
	}
	if got, _, err := copyCandidates("한글 여행", 1); err != nil || len(got) != 1 || len(got[0]) != 1 {
		t.Fatalf("one-line style offered a wrap: %v %v", got, err)
	}
	// Every bundled file is pinned by size and checksum (CDS-17, CLIP-13): a
	// swapped one fails the constructor, not the render.
	for key := range bundledFontPaths(t) {
		cfg := r.cfg
		cfg.FontPaths = maps.Clone(cfg.FontPaths)
		path := filepath.Join(t.TempDir(), "font.ttf")
		_ = os.WriteFile(path, []byte("fake"), 0600)
		cfg.FontPaths[key] = path
		if _, err := NewRenderer(r.media, cfg); err == nil {
			t.Fatalf("a wrong %s file was accepted", key)
		}
	}
	// Every face CDS-17 names is bundled, and a role — or a caption style
	// wearing one — is set in the file its own face and weight name.
	for _, face := range []string{"pretendard", "paperlogy", "jua", "nanummyeongjo"} {
		if r.face(design.TypeRole{Face: face}) == nil || r.family(design.TypeRole{Face: face}) != design.FontFamily(face) {
			t.Fatalf("face %q did not resolve", face)
		}
	}
	if r.face(design.Type["hook"]) != r.fonts["paperlogy"] || r.face(design.Type["body"]) != r.fonts["pretendard"] {
		t.Fatal("the coverage check reads the wrong face")
	}
	// NanumMyeongjo is the one face bundled at two weights, and each weight is
	// its own file under one family (CDS-17).
	serif := design.TypeRole{Face: "nanummyeongjo"}
	light, heavy := serif, serif
	light.Weight, heavy.Weight = 400, 800
	if r.face(light) != r.fonts["nanummyeongjo"] || r.face(heavy) != r.fonts["nanummyeongjo-800"] || r.face(light) == r.face(heavy) {
		t.Fatal("the two NanumMyeongjo weights did not resolve to their own files")
	}
	// And it reads it per text, so a character only SOME faces carry is refused
	// exactly where it cannot be drawn (CLIP-13): Paperlogy has no ♥ or 〃.
	for _, text := range []string{"♥", "〃"} {
		if err := r.checkCopy(text, design.Type["body"]); err != nil {
			t.Fatalf("Pretendard carries %q: %v", text, err)
		}
		if err := r.checkCopy(text, design.Type["title"]); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("Paperlogy does not carry %q, so it may not be substituted: %v", text, err)
		}
	}
	// Jua carries 2,367 of the 11,172 Hangul syllables, so an ordinary sentence
	// sets and an uncommon syllable does not (CDS-84). 쎯 is one it lacks.
	jua := design.CaptionStyle{Face: "jua", Weight: 400}.Role()
	if got := r.MissingGlyph("여기 진짜 좋아요", jua); got != 0 {
		t.Fatalf("Jua could not set an ordinary sentence: %q", got)
	}
	if got := r.MissingGlyph("쎯은 없다", jua); got != '쎯' {
		t.Fatalf("Jua reported %q rather than the syllable it lacks", got)
	}
	if got := r.MissingGlyph("쎯은 없다", design.DefaultCaption().Role()); got != 0 {
		t.Fatalf("the default style's face lacks %q, so nothing could fall back to it", got)
	}
}

// The four styles on the three ratios, each drawn only from CDS tokens. The
// goldens are what catches a token silently changing shape; the assertions after
// them are the properties CDS states outright.
//
// staticCaptionStyles are the approved styles one rasterisation covers: the
// sequence-rendered ones draw a layer per output frame and are pinned by their
// own goldens.
func staticCaptionStyles() []design.CaptionStyle {
	out := []design.CaptionStyle{}
	for _, style := range design.CaptionStyles() {
		if style.Static() {
			out = append(out, style)
		}
	}
	return out
}

func TestCopyLayoutAndSVGGolden(t *testing.T) {
	text := `한글 & <여행>`
	bounds := map[string]clip.Region{text: {X: 1, Y: -80, Width: 500, Height: 100}}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		canvas, _ := clip.ClipCanvas(ratio)
		for _, caption := range staticCaptionStyles() {
			style, rule := caption.ID, caption.Rule()
			c := clip.Copy{Text: text, Anchor: rule.Anchor, Align: rule.Align, Style: style, Accent: "coral"}
			l, err := fitCopy(canvas, c, [][]string{{text}}, bounds)
			if err != nil {
				t.Fatalf("%s/%s: %v", ratio, style, err)
			}
			svg := copySVG(canvas, c, l, Luminance{})
			golden(t, fmt.Sprintf("copy-%s-%s.svg", style, ratio), svg+"\n")
			if strings.Contains(svg, "<여행>") || !strings.Contains(svg, "&amp; &lt;여행&gt;") {
				t.Fatal("text not escaped")
			}
			// CDS-2: the plate the layout chose is inside the safe area, and the
			// size never falls below the role's floor (CDS-3, CDS-19).
			if l.Region.X < canvas.Safe.X || l.Region.X+l.Region.Width > canvas.Safe.X+canvas.Safe.Width {
				t.Fatalf("%s/%s left the safe area: %+v", ratio, style, l.Region)
			}
			if l.FontSize < caption.Role().Min || l.FontSize > caption.Role().Size {
				t.Fatalf("%s/%s size %v outside [%v,%v]", ratio, style, l.FontSize, caption.Role().Min, caption.Role().Size)
			}
			// Each style is set in the face it names, at its own weight, and in
			// nothing else (CDS-18, CDS-80).
			if want := fmt.Sprintf(`font-family="%s" font-size="%.0f" font-weight="%d"`, design.FontFamily(caption.Face), l.FontSize, caption.Weight); !strings.Contains(svg, want) {
				t.Fatalf("%s/%s is not set in its own face: want %s", ratio, style, want)
			}
			// A plated style paints its plate and no stroke; an unplated one the
			// reverse, with the drop shadow (CDS-23..26).
			plate := strings.Contains(svg, `fill="`+design.Color[rule.Plate].Hex+`" fill-opacity=`)
			if (rule.Plate != "") != plate {
				t.Fatalf("%s/%s plate=%v", ratio, style, plate)
			}
			if strings.Contains(svg, "feDropShadow") != (rule.Shadow != "") {
				t.Fatalf("%s/%s shadow", ratio, style)
			}
			if want := fmt.Sprintf(`stroke-width="%.3f"`, rule.StrokeWidth()); !strings.Contains(svg, want) {
				t.Fatalf("%s/%s missing %s", ratio, style, want)
			}
		}
	}
	canvas, _ := clip.ClipCanvas("vertical")
	// Neutral: no accent means no bar, no dot and no highlight anywhere.
	for _, caption := range staticCaptionStyles() {
		style := caption.ID
		c := clip.Copy{Text: text, Anchor: "bottom", Align: "center", Style: style}
		l, err := fitCopy(canvas, c, [][]string{{text}}, bounds)
		if err != nil {
			t.Fatal(err)
		}
		svg := copySVG(canvas, c, l, Luminance{})
		golden(t, "copy-"+style+"-neutral.svg", svg+"\n")
		if strings.Contains(svg, design.Accent["coral"]) || strings.Contains(svg, "<circle") {
			t.Fatalf("%s painted an accent it was not given", style)
		}
	}
	// A keyword at the end of the line: its highlight is measured from the
	// prefix's advance and still may not cross the safe area's right edge (920).
	keyed := "가격 9900원"
	kb := map[string]clip.Region{keyed: {X: 1, Y: -80, Width: 700, Height: 100}, "가격 ": {X: 1, Y: -80, Width: 220, Height: 100}, "9900원": {X: 1, Y: -80, Width: 470, Height: 100}}
	for _, style := range []string{"bold"} {
		c := clip.Copy{Text: keyed, Keyword: "9900원", Anchor: design.Caption().Anchor, Align: "right", Style: style, Accent: "amber"}
		l, err := fitCopy(canvas, c, [][]string{{keyed}}, kb)
		if err != nil {
			t.Fatal(err)
		}
		if !l.Keyword.Present || l.Keyword.Line != 0 || l.Keyword.Offset <= 0 {
			t.Fatalf("%s lost its measured keyword: %+v", style, l.Keyword)
		}
		svg := copySVG(canvas, c, l, Luminance{})
		golden(t, "copy-"+style+"-keyword.svg", svg+"\n")
		for _, e := range l.Elements(0, 0, c, 0, 3000) {
			if e.Region.X+e.Region.Width > canvas.Safe.X+canvas.Safe.Width {
				t.Fatalf("%s %s crossed x %v: %+v", style, e.Kind, canvas.Safe.X+canvas.Safe.Width, e.Region)
			}
		}
		if strings.Contains(svg, `fill-opacity="0.9"`) != design.Caption().Highlight {
			t.Fatalf("%s highlight", style)
		}
		// 크게 강조 colours the word in place instead (CDS-25).
		if strings.Contains(svg, "<tspan") != (style == "bold") {
			t.Fatalf("%s accent word", style)
		}
	}
	// Two lines, and the fit loop refusing rather than shrinking past the floor.
	c := clip.Copy{Text: "one two", Style: "clean", Anchor: "top", Align: "left"}
	b := map[string]clip.Region{"one two": {Width: 2500, Height: 100}, "one ": {Width: 800, Height: 100}, "two": {Width: 800, Height: 100}}
	l, err := fitCopy(canvas, c, [][]string{{"one two"}, {"one ", "two"}}, b)
	if err != nil || len(l.Lines) != 2 || l.FontSize != design.Type["title"].Size {
		t.Fatalf("%+v %v", l, err)
	}
	for style, rule := range map[string]design.StyleRule{"bold": design.Caption()} {
		wide := map[string]clip.Region{text: {Width: 100000, Height: 100}}
		c := clip.Copy{Text: text, Anchor: rule.Anchor, Align: rule.Align, Style: style}
		if _, err := fitCopy(canvas, c, [][]string{{text}}, wide); !errors.Is(err, clip.ErrCopyTooLong) {
			t.Fatalf("%s shrank below its floor: %v", style, err)
		}
		// A copy that fits ONLY at the floor still renders, exactly there. The
		// room available depends on the style's own anchor and alignment, so it
		// is probed rather than assumed.
		floor := rule.Role().Min
		left, right, _ := copyInsets(rule)
		room := 0.0
		for w := canvas.Safe.Width; w >= 1; w-- {
			if _, err := clip.PlaceCopy(canvas, rule.Anchor, rule.Align, w, 2*floor); err == nil {
				room = w
				break
			}
		}
		atFloor := map[string]clip.Region{text: {Width: (room - left - right - 1) * 100 / floor, Height: 100}}
		got, err := fitCopy(canvas, c, [][]string{{text}}, atFloor)
		if err != nil || got.FontSize != floor {
			t.Fatalf("%s did not reach its floor %v: %v %v", style, floor, got.FontSize, err)
		}
	}
}
func golden(t *testing.T, name, actual string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_CLIP_GOLDENS") == "1" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual != string(want) {
		t.Fatalf("golden %s mismatch\n%s", name, actual)
	}
}

// What the sampled ground changes in the SVG: a scrim under an unplated style,
// and an accent word that turns white on a bright one (CDS-32, CDS-44).
func TestScrimAndAccentOnASampledGround(t *testing.T) {
	text, keyed := "한글 & <여행>", "가격 9900원"
	bounds := map[string]clip.Region{
		text: {X: 1, Y: -80, Width: 500, Height: 100}, keyed: {X: 1, Y: -80, Width: 700, Height: 100},
		"가격 ": {X: 1, Y: -80, Width: 220, Height: 100}, "9900원": {X: 1, Y: -80, Width: 470, Height: 100},
	}
	canvas, _ := clip.ClipCanvas("vertical")
	bright := Luminance{Mean: 0.8, R: 0.9, G: 0.9, B: 0.9, Frames: []float64{0.8}}
	busy := Luminance{Mean: 0.2, Sigma: 0.4, R: 0.2, G: 0.2, B: 0.2, Frames: []float64{0.2}}
	dark := Luminance{Mean: 0.1, R: 0.1, G: 0.1, B: 0.1, Frames: []float64{0.1}}
	for _, style := range []string{"bold"} {
		rule := design.Caption()
		c := clip.Copy{Text: keyed, Keyword: "9900원", Anchor: rule.Anchor, Align: rule.Align, Style: style, Accent: "amber"}
		l, err := fitCopy(canvas, c, [][]string{{keyed}}, bounds)
		if err != nil {
			t.Fatal(err)
		}
		svg := copySVG(canvas, c, l, bright)
		golden(t, "copy-"+style+"-bright.svg", svg+"\n")
		// A plate is its own ground, so a plated style never carries a scrim
		// (CDS-32); an unplated one does, at its anchor's own edge.
		edge := "top"
		if rule.Anchor == "bottom" || rule.Anchor == "lower_mid" {
			edge = "bottom"
		}
		scrimmed := strings.Contains(svg, `fill="url(#scrim)"`)
		if scrimmed != (rule.Plate == "") {
			t.Fatalf("%s scrim=%v", style, scrimmed)
		}
		if scrimmed && !strings.Contains(svg, `stop-color="`+design.Scrim[edge].Hex+`" stop-opacity="`+strconv.FormatFloat(design.Scrim[edge].From, 'f', -1, 64)) {
			t.Fatalf("%s did not paint the %s gradient: %s", style, edge, svg)
		}
		// On a bright ground 크게 강조's accent WORD turns white (CDS-44); 형광펜's
		// marker stroke keeps the accent, because it sits behind white text.
		if rule.Stroke != "" && !rule.Highlight && strings.Contains(svg, `<tspan fill="`+design.Accent["amber"]) {
			t.Fatalf("%s kept its accent word on a bright ground", style)
		}
		if rule.Highlight && !strings.Contains(svg, design.Accent["amber"]) {
			t.Fatalf("%s whitened the marker stroke behind its white text", style)
		}
		// Busy but dark footage gets the scrim and keeps its accent.
		onBusy := copySVG(canvas, c, l, busy)
		if strings.Contains(onBusy, `fill="url(#scrim)"`) != (rule.Plate == "") {
			t.Fatalf("%s ignored σ", style)
		}
		if rule.Highlight && !strings.Contains(onBusy, design.Accent["amber"]) {
			t.Fatalf("%s lost its highlight on a dark ground", style)
		}
		// A dark, even ground asks for neither.
		if onDark := copySVG(canvas, c, l, dark); strings.Contains(onDark, "url(#scrim)") {
			t.Fatalf("%s scrimmed a dark ground", style)
		}
	}
	// An unsampled ground draws exactly what it drew before the sampler existed.
	c := clip.Copy{Text: text, Anchor: "bottom", Align: "center", Style: "mark", Accent: "amber"}
	l, err := fitCopy(canvas, c, [][]string{{text}}, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(copySVG(canvas, c, l, Luminance{}), "scrim") {
		t.Fatal("no sample, no scrim")
	}
}

func TestRenderFilterGoldens(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	canvas, _ := clip.ClipCanvas("horizontal")
	c := clip.EditCut{StartMS: 100, EndMS: 7700, Focal: clip.Point{X: .25, Y: .75}, Volume: volume(.5)}
	golden(t, "cut.filter", cutGraph(r.cfg, canvas, c, clip.MediaInfo{HasAudio: true}, 228, layers{Copies: []string{"copy.png"}}, true, true)+"\n")
	// CDS-36 on one timeline: a hard cut CONCATENATES and a scene change
	// dissolves, so the same three cuts join three different ways. The audio
	// graph is golden beside each one because its boundaries are CDS-35's, not
	// the picture's — a hard cut still cross-fades, over 60 ms it does not pay
	// for on the timeline.
	for _, plan := range []struct {
		name        string
		transitions []int
	}{
		{"cut", []int{0, 0, 0}},
		{"fade", []int{0, 200, 200}},
		{"mixed", []int{0, 0, 200}},
		{"black", []int{0, 300, 0}},
	} {
		golden(t, "composition-"+plan.name+".filter", compositionGraph(r.cfg, []int{156, 150, 156}, plan.transitions, "yuv420p")+"\n")
		golden(t, "composition-"+plan.name+".audio.filter", compositionAudioGraph(r.cfg, []int{156, 150, 156}, plan.transitions, 3)+"\n")
	}
	if strings.Contains(cutGraph(r.cfg, canvas, c, clip.MediaInfo{}, 228, layers{}, false, false), "[a]") {
		t.Fatal("invented audio")
	}
	if !strings.Contains(cutGraph(r.cfg, canvas, c, clip.MediaInfo{}, 228, layers{}, true, true), "anullsrc=r=48000:cl=stereo") {
		t.Fatal("missing synthesized silence")
	}
	// CDS-4 and CDS-27: exactly one 180 ms fade-in settling 12 px, one 120 ms
	// fade-out that does not move, and a window inset 120 ms at both ends.
	graph := cutGraph(r.cfg, canvas, c, clip.MediaInfo{HasAudio: true}, 228, layers{Copies: []string{"copy.png"}}, true, true)
	for _, want := range []string{
		"fade=t=in:st=0.120:d=0.180:alpha=1",
		"fade=t=out:st=7.360:d=0.120:alpha=1",
		"overlay=x=0:y='12*pow(1-min(1,max(0,(t-0.120)/0.180)),3)'",
		"enable='gte(t,0.120)*lt(t,7.480)'",
	} {
		if !strings.Contains(graph, want) {
			t.Fatalf("lost motion %s in %s", want, graph)
		}
	}
	// The fixed layer is its own overlay with no fade and no y expression: the
	// disclosure badge may not move (CDS-31) while the copy must (CDS-4).
	both := cutGraph(r.cfg, canvas, c, clip.MediaInfo{}, 228, layers{Fixed: "fixed.png", Copies: []string{"copy.png"}}, false, false)
	if !strings.Contains(both, "[base][1:v:0]overlay=0:0:format=auto:shortest=0[fixed];") || !strings.Contains(both, "[2:v:0]format=rgba,loop=loop=227:size=1:start=0,fade=") || !strings.Contains(both, "[fixed][plate0]overlay=x=0:y=") {
		t.Fatalf("fixed and animated layers are not separate: %s", both)
	}
	fixedOnly := cutGraph(r.cfg, canvas, clip.EditCut{StartMS: 100, EndMS: 7700}, clip.MediaInfo{}, 228, layers{Fixed: "fixed.png"}, false, false)
	if strings.Contains(fixedOnly, "fade=") || !strings.Contains(fixedOnly, "[fixed]trim=") {
		t.Fatalf("a cut with no copy animated its badge: %s", fixedOnly)
	}
	// Nothing else moves or eases: no zoom, wipe, slide, rotation or blur.
	for _, forbidden := range []string{"zoompan", "rotate", "boxblur", "gblur", "wipe", "slide", "scroll"} {
		if strings.Contains(graph, forbidden) {
			t.Fatalf("forbidden motion %s", forbidden)
		}
	}
	// An explicit window is exactly what the plan asked for, not re-inset.
	explicit := c
	explicit.Copies = []clip.Copy{{Text: "x", StartMS: 1000, EndMS: 4000}}
	if !strings.Contains(cutGraph(r.cfg, canvas, explicit, clip.MediaInfo{}, 228, layers{Copies: []string{"copy.png"}}, false, false), "enable='gte(t,1.000)*lt(t,4.000)'") {
		t.Fatal("an explicit caption window was moved")
	}
	frames := cutFrames(clip.EditPlan{Cuts: []clip.EditCut{{EndMS: 5011}, {EndMS: 5022}, {EndMS: 5367}}}, 30)
	if !reflect.DeepEqual(frames, []int{150, 151, 161}) {
		t.Fatal(frames)
	}
}

func TestCaptionExposureUsesOnlyValidatedCutRelativeTimes(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	canvas, _ := clip.ClipCanvas("vertical")
	c := clip.Cut{EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Caption{{StartMS: 1000, EndMS: 12000}}}
	graph := cutGraph(r.cfg, canvas, c, clip.MediaInfo{}, 450, layers{Copies: []string{"copy.png"}}, false, false)
	if !strings.Contains(graph, ":enable='gte(t,1.000)*lt(t,12.000)'") {
		t.Fatal(graph)
	}
}
func TestCopyMeasurementAndExplicitFontArguments(t *testing.T) {
	fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) { return []byte("m0,1,120,500,100\n"), nil }}
	a := newAdapter(t, fake)
	r := testRenderer(t, a)
	if err := a.WithWorkspace(t.Context(), "copy", func(ws clip.MediaWorkspace) error {
		bounds, err := r.measure(t.Context(), ws, []string{"한글"}, 600, 0, fontFamily)
		if err != nil {
			return err
		}
		if bounds["한글"].Y != -80 {
			t.Fatal(bounds)
		}
		args := fake.calls[0].Args
		if !slices.Contains(args, "--skip-system-fonts") || !slices.Contains(args, "--query-all") || slices.ContainsFunc(slices.Collect(maps.Values(r.cfg.FontPaths)), func(p string) bool { return !slices.Contains(args, p) }) {
			t.Fatal(args)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRenderDoesNotLoadInvalidPlansOrLeakOnFailure(t *testing.T) {
	for _, mode := range []string{"invalid", "loader", "runner", "panic", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			fake := &fakeRunner{run: func(context.Context, Command) ([]byte, error) { return nil, fmt.Errorf("ffmpeg failed") }}
			a := newAdapter(t, fake)
			r := testRenderer(t, a)
			s := clip.RenderSource{ID: "source", Fingerprint: "hash", Info: clip.MediaInfo{DurationMS: 20000, Width: 1920, Height: 1080}}
			plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.EditCut{{ID: "one", SourceID: s.ID, Fingerprint: s.Fingerprint, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Style: "clean", Anchor: "bottom", Align: "center"}}}}}
			if mode == "invalid" {
				plan.DurationMS = 14000
			}
			var workspace string
			loaded := false
			func() {
				defer func() {
					if recovered := recover(); recovered != nil && mode != "panic" {
						t.Fatal(recovered)
					}
				}()
				err := a.WithWorkspace(t.Context(), "render", func(ws clip.MediaWorkspace) error {
					workspace = ws.Path
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					if mode == "cancel" {
						cancel()
					}
					_, err := r.Render(ctx, ws, plan, []clip.RenderSource{s}, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
						loaded = true
						if mode == "loader" {
							return errors.New("download failed")
						}
						if mode == "panic" {
							panic("loader panicked")
						}
						return consume(clip.MediaSource{SourceID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info, Path: sourceFile(t, ws)})
					})
					return err
				})
				if err == nil {
					t.Fatal("failure accepted")
				}
			}()
			if (mode == "invalid" || mode == "cancel") && loaded {
				t.Fatal("loaded invalid plan")
			}
			if _, err := os.Stat(workspace); !os.IsNotExist(err) {
				t.Fatal("workspace leaked")
			}
		})
	}
}
