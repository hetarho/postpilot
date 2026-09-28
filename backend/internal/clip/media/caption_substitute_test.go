package media

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Paperlogy maps 갂 to a glyph with no outline, so the cmap alone called it
// covered and a 크게 강조 caption drew it blank. 뷁 it does draw (CDS-84).
const paperlogyGap = "갂"

var substituteTspan = `<tspan font-family="` + design.FontFamily("wantedsans") + `">`

// A face covers only what it draws, and a caption's check is "its face, or the
// substitute", while every other text keeps its face alone (CDS-84, CDS-77).
func TestCoverageReadsInkAndACaptionMayUseTheSubstitute(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	bold := design.DefaultCaption().Role()
	if got := r.MissingGlyph(paperlogyGap+" 좋아", bold); got != '갂' {
		t.Fatalf("Paperlogy's empty 갂 counted as drawn: %q", got)
	}
	if got := r.MissingGlyph("뷁 좋아", bold); got != 0 {
		t.Fatalf("Paperlogy draws 뷁 but reported %q", got)
	}
	if err := r.checkCopy(paperlogyGap, design.Type["hook"]); !errors.Is(err, clip.ErrInvalid) {
		t.Fatalf("a region role took a substitute: %v", err)
	}
	if err := r.checkCaption(paperlogyGap+" 좋아", bold); err != nil {
		t.Fatalf("a caption could not use the substitute: %v", err)
	}
	if got := r.substitutes(paperlogyGap+" 좋아 "+paperlogyGap, bold); !maps.Equal(got, map[rune]bool{'갂': true}) {
		t.Fatalf("substitutes = %v", got)
	}
	if got := r.substitutes("여기 진짜 좋아요", bold); got != nil {
		t.Fatalf("a caption its face draws whole substituted %v", got)
	}
	keynote, _ := design.LookupCaptionStyle("keynote")
	if got := r.substitutes(paperlogyGap, keynote.Role()); got != nil {
		t.Fatalf("Wanted Sans substituted for itself: %v", got)
	}
	// ㎏ is drawn by Paperlogy and by neither Jua nor Wanted Sans: a Jua
	// caption holding it still has to leave its style.
	jua := design.CaptionStyle{Face: "jua", Weight: 400}.Role()
	if got := r.MissingCaptionGlyph("3㎏", jua); got != '㎏' {
		t.Fatalf("Jua caption missing = %q", got)
	}
	if got := r.MissingCaptionGlyph("3㎏", bold); got != 0 {
		t.Fatalf("the default style cannot take ㎏: %q", got)
	}
}

func TestCaptionMarkupRunsTheSubstitute(t *testing.T) {
	if got := measureSVG([]string{"a<b>"}, 800, 0, "Paperlogy", nil); !strings.Contains(got, `>a&lt;b&gt;</text>`) {
		t.Fatalf("plain measurement changed: %s", got)
	}
	got := measureSVG([]string{"갂갂 좋다 쎯"}, 800, 0, "Paperlogy", map[rune]bool{'갂': true, '쎯': true})
	if want := `>` + substituteTspan + `갂갂</tspan> 좋다 ` + substituteTspan + `쎯</tspan></text>`; !strings.Contains(got, want) {
		t.Fatalf("measurement markup:\n%s\nwant %s", got, want)
	}
}

// CDS-84: the caption keeps its own style — static, sequence-rendered and
// per-word alike — with the character its face does not draw in the
// substitute, and no notice.
func TestACaptionKeepsItsStyleAndSetsTheCharacterInTheSubstitute(t *testing.T) {
	for _, tc := range []struct{ style, text, run string }{
		{"bold", paperlogyGap + "이 최고", paperlogyGap},
		{"glitch", paperlogyGap + "이 최고", paperlogyGap},
		{"pop", juaGap + "이 최고", "똠"},
	} {
		plan := declaredPlan(t, captionBody(tc.text), "vertical")
		plan.CaptionStyles = []string{tc.style}
		layout := measuredDeclared(t, plan)
		visual := captionVisual(t, layout)
		if visual.glyphFallback || visual.copy.Style != tc.style || visual.caption.Caption.ID != tc.style {
			t.Fatalf("%s: left its style for %q", tc.style, visual.copy.Style)
		}
		if !visual.caption.Substitute[[]rune(tc.run)[0]] {
			t.Fatalf("%s: substitute set %v", tc.style, visual.caption.Substitute)
		}
		canvas, _ := clip.ClipCanvas("vertical")
		_, r := measured(t)
		svg, err := r.captionDocument(canvas, visual)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(svg, substituteTspan+tc.run) {
			t.Fatalf("%s: the character was not set in the substitute: %s", tc.style, svg)
		}
		if !strings.Contains(svg, `font-family="`+design.FontFamily(visual.caption.Caption.Face)+`"`) {
			t.Fatalf("%s: the rest left its own face", tc.style)
		}
		layout.recordContrastNotices()
		for _, notice := range layout.plan.Notices {
			if notice.Reason == "composition_caption_glyph" {
				t.Fatalf("%s: a substitution recorded a notice", tc.style)
			}
		}
	}
}

// A character neither the face nor the substitute draws still sends the
// caption to the default style, by name (CDS-84, CLIP-108).
func TestACaptionFallsBackWhenNeitherFaceDrawsACharacter(t *testing.T) {
	plan := declaredPlan(t, captionBody("한우 3㎏"), "vertical")
	plan.CaptionStyles = []string{"pop"}
	layout := measuredDeclared(t, plan)
	visual := captionVisual(t, layout)
	if !visual.glyphFallback || visual.copy.Style != design.DefaultCaptionStyle {
		t.Fatalf("Jua kept ㎏: style %q fallback %v", visual.copy.Style, visual.glyphFallback)
	}
	layout.recordContrastNotices()
	want := clip.PlanNotice{CopyFallback: clip.CopyFallback{ElementID: visual.manifest.ElementID, CutID: visual.manifest.CutID, Reason: "composition_caption_glyph"}, Action: "style_fallback"}
	found := false
	for _, notice := range layout.plan.Notices {
		found = found || notice == want
	}
	if !found {
		t.Fatalf("no notice named the caption that fell back: %+v", layout.plan.Notices)
	}
}

// The static template keeps a coloured keyword's accent across the split.
func TestTheStaticTemplateColoursAKeywordAcrossTheSubstitute(t *testing.T) {
	text := "맛집 " + paperlogyGap + "집"
	canvas, _ := clip.ClipCanvas("vertical")
	c := clip.Copy{Text: text, Style: "bold", Anchor: "bottom", Align: "center", Accent: "coral", Keyword: paperlogyGap + "집"}
	bounds := map[string]clip.Region{}
	for _, v := range []string{text, c.Keyword, text} {
		bounds[v] = clip.Region{X: 1, Y: -80, Width: 40 * float64(len([]rune(v))), Height: 100}
	}
	l, err := fitCopy(canvas, c, [][]string{{text}}, bounds)
	if err != nil {
		t.Fatal(err)
	}
	l.Substitute = map[rune]bool{'갂': true}
	svg := copySVG(canvas, c, l, Luminance{})
	accent := design.Accent["coral"]
	if want := `맛집 <tspan font-family="` + design.FontFamily("wantedsans") + `" fill="` + accent + `">갂</tspan><tspan fill="` + accent + `">집</tspan></text>`; !strings.Contains(svg, want) {
		t.Fatalf("keyword runs:\n%s\nwant %s", svg, want)
	}
	l.Substitute = nil
	if plain := copySVG(canvas, c, l, Luminance{}); !strings.Contains(plain, `맛집 <tspan fill="`+accent+`">갂집</tspan></text>`) {
		t.Fatalf("a caption with nothing to substitute changed: %s", plain)
	}
}

// A copy template that ignores runs would draw Paperlogy's empty glyph, so the
// renderer refuses it at boot (CDS-84).
func TestRendererRefusesACopyTemplateWithoutRuns(t *testing.T) {
	dir := overlayDirectory(t)
	path := filepath.Join(dir, "caption", "overlay.svg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	start, end := strings.Index(string(data), "{{if .Runs}}"), strings.Index(string(data), "{{else if .Colored}}")
	if start < 0 || end < start {
		t.Fatal("the caption template has no runs branch")
	}
	old := string(data[:start]) + "{{if .Colored}}" + string(data[end+len("{{else if .Colored}}"):])
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	a := newAdapter(t, &fakeRunner{})
	cfg := testRenderer(t, a).cfg
	cfg.OverlayDir = dir
	if _, err := NewRenderer(a, cfg); err == nil || !strings.Contains(err.Error(), "substituted caption run") {
		t.Fatalf("a template without runs was accepted: %v", err)
	}
	// The operator example's caption template, in an otherwise current
	// catalog, draws runs too.
	example, err := os.ReadFile("../../../examples/overlays/shorts-editorial/caption/overlay.svg")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, example, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRenderer(a, cfg); err != nil {
		t.Fatalf("the example caption template does not draw runs: %v", err)
	}
}
