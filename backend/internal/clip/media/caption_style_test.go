package media

import (
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Jua carries 2,367 of the 11,172 Hangul syllables (CDS-84) — roughly KS X
// 1001 — so an ordinary sentence sets and 똠얌꿍 does not.
const juaGap = "똠얌꿍"

func captionBody(text string) string {
	return `<clip version="1" intro="a" caption="bold" outro="e">` +
		`<text id="one" kind="fixed" role="caption" basis="whole">` + text + `</text>` +
		`<text id="empty-hook" kind="fixed" role="hook" basis="output-start"/>` +
		`<text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
}

func captionVisual(t *testing.T, layout declaredLayout) declaredVisual {
	t.Helper()
	for _, visual := range layout.visuals {
		if visual.text.Resolved.Element.Role == "caption" {
			return visual
		}
	}
	t.Fatal("the fixture laid out no caption")
	return declaredVisual{}
}

// A style whose face can set the caption draws it in that face, at that weight.
func TestACaptionIsSetInTheFaceItsStyleNames(t *testing.T) {
	for _, id := range []string{"bold", "keynote", "film", "pop"} {
		style, ok := design.LookupCaptionStyle(id)
		if !ok {
			t.Fatal(id)
		}
		plan := declaredPlan(t, captionBody("여기 진짜 좋아요"), "vertical")
		plan.CaptionStyles = []string{id}
		layout := measuredDeclared(t, plan)
		visual := captionVisual(t, layout)
		if visual.copy.Style != id || visual.caption.Caption.ID != id {
			t.Fatalf("%s: laid out as %q", id, visual.copy.Style)
		}
		if visual.caption.Role.Face != style.Face || visual.caption.Role.Weight != style.Weight {
			t.Fatalf("%s: set in %+v", id, visual.caption.Role)
		}
		canvas, _ := clip.ClipCanvas("vertical")
		_, r := measured(t)
		svg, err := r.declaredSVG(canvas, visual)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(svg, `font-family="`+design.FontFamily(style.Face)+`"`) {
			t.Fatalf("%s was not drawn in %s: %s", id, style.Face, svg)
		}
		// No glyph is ever taken from another family, so no other face's name
		// may appear anywhere in the layer this style draws.
		for face, family := range design.Faces {
			if face != style.Face && strings.Contains(svg, `font-family="`+family+`"`) {
				t.Fatalf("%s reached for %s", id, family)
			}
		}
	}
}

// CDS-84: a face that cannot set one syllable loses that caption to the default
// style, with a CLIP-108 notice naming it — and keeps every other caption.
func TestACaptionFallsBackWhenItsFaceLacksASyllable(t *testing.T) {
	plan := declaredPlan(t, captionBody(juaGap+"이 최고"), "vertical")
	plan.CaptionStyles = []string{"pop"}
	layout := measuredDeclared(t, plan)
	visual := captionVisual(t, layout)
	if !visual.glyphFallback {
		t.Fatal("Jua set a syllable it has no glyph for")
	}
	if visual.copy.Style != design.DefaultCaptionStyle {
		t.Fatalf("the caption fell back to %q rather than to the default style", visual.copy.Style)
	}
	if visual.caption.Role.Face != design.DefaultCaption().Face {
		t.Fatalf("the fallback was not set in the default style's face: %+v", visual.caption.Role)
	}
	layout.recordContrastNotices()
	want := clip.PlanNotice{CopyFallback: clip.CopyFallback{ElementID: visual.manifest.ElementID, CutID: visual.manifest.CutID, Reason: "composition_caption_glyph"}, Action: "style_fallback"}
	if !slices.Contains(layout.plan.Notices, want) {
		t.Fatalf("no notice named the caption that fell back: %+v", layout.plan.Notices)
	}
	// The same project's other caption keeps the style it asked for: the swap
	// is one caption's, never the project's.
	kept := declaredPlan(t, captionBody("여기 진짜 좋아요"), "vertical")
	kept.CaptionStyles = []string{"pop"}
	keptLayout := measuredDeclared(t, kept)
	if visual := captionVisual(t, keptLayout); visual.glyphFallback || visual.copy.Style != "pop" {
		t.Fatalf("a caption Jua can set was taken away from it: %q", visual.copy.Style)
	}
	keptLayout.recordContrastNotices()
	for _, notice := range keptLayout.plan.Notices {
		if notice.Reason == "composition_caption_glyph" {
			t.Fatal("a caption that set cleanly still recorded a glyph notice")
		}
	}
}

func TestCaptionStyleResolutionKeepsOwnerNarrationAndLegacyDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, narration, owner, want string
		styles                       []string
		notice                       bool
	}{
		{"narration", "film", "", "film", []string{"keynote", "film", "pop"}, false},
		{"owner wins", "film", "pop", "pop", []string{"keynote", "film", "pop"}, false},
		{"legacy automatic", "auto", "", "keynote", []string{"keynote", "film"}, false},
		{"legacy absent", "", "", "keynote", []string{"keynote", "film"}, false},
		{"single style", "auto", "", "film", []string{"film"}, false},
		{"empty selection", "auto", "", "bold", nil, false},
		{"narrowed selection", "film", "", "keynote", []string{"keynote"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caption := narrationText("narration-1", "여기 좋아요", 1000, 5000)
			caption.Resolved.Element.Style, caption.Owner.Style = tc.narration, tc.owner
			plan := narrationPlan(t, caption)
			plan.CaptionStyles = tc.styles
			layout := measuredDeclared(t, plan)
			visual := captionVisual(t, layout)
			if visual.copy.Style != tc.want {
				t.Fatalf("style = %q, want %q", visual.copy.Style, tc.want)
			}
			layout.recordContrastNotices()
			layout.recordContrastNotices() // A repeated layout must not duplicate the notice.
			count := 0
			for _, n := range layout.plan.Notices {
				if n.Reason == "composition_caption_style" {
					count++
					if n.ElementID != "narration-1" || n.Action != "style_fallback" {
						t.Fatal(n)
					}
				}
			}
			want := 0
			if tc.notice {
				want = 1
			}
			if count != want {
				t.Fatalf("notices = %d, want %d", count, want)
			}
		})
	}
}

func TestFrozenLegacyCaptionKeepsTheSelectionsFirstStyle(t *testing.T) {
	// A legacy freeze carries its old copy style in Element.Style. Prior
	// renders ignored it, so teaching narration to use that field must not
	// restyle an existing frozen legacy plan.
	plan := declaredPlan(t, captionBody("여기 좋아요"), "vertical")
	plan.Portable.Snapshot.Legacy = true
	plan.CaptionStyles = []string{"keynote", "film"}
	for i := range plan.Portable.Elements {
		if plan.Portable.Elements[i].Resolved.Element.Role == "caption" {
			plan.Portable.Elements[i].Resolved.Element.Style = "film"
		}
	}
	if visual := captionVisual(t, measuredDeclared(t, plan)); visual.copy.Style != "keynote" {
		t.Fatal("a frozen legacy caption changed style", visual.copy.Style)
	}
}
