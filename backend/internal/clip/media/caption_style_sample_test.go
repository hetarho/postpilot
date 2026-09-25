package media

import (
	"fmt"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// ① offers the caption styles by their own look, drawn by the renderer itself:
// one sample per approved style, each labelled with the style it samples and
// with whether that style draws a frame at a time (CDS-80, CDS-81, CDS-83).
func TestEveryApprovedStyleIsSampledByTheRendererItself(t *testing.T) {
	styles := []string{}
	sequence := map[string]bool{}
	for _, style := range design.CaptionStyles() {
		styles = append(styles, style.ID)
		sequence[style.ID] = !style.Static()
	}
	plan, sources, err := clip.CaptionStyleSamplePlan("vertical", styles)
	if err != nil {
		t.Fatal(err)
	}
	_, r := measured(t)
	out, err := r.CaptionFragments(t.Context(), plan, sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(styles) {
		t.Fatalf("%d samples for %d styles", len(out), len(styles))
	}
	seen := map[string]bool{}
	for i, fragment := range out {
		if fragment.Style != styles[i] {
			t.Fatalf("sample %d is %q, want %q — the offer order is the registry's", i, fragment.Style, styles[i])
		}
		if seen[fragment.Style] {
			t.Fatal("a style was sampled twice", fragment.Style)
		}
		seen[fragment.Style] = true
		if fragment.Sequence != sequence[fragment.Style] {
			t.Fatalf("%s: sample says sequence=%v, the registry says %v", fragment.Style, fragment.Sequence, sequence[fragment.Style])
		}
		if fragment.Box.Width <= 0 || fragment.Box.Height <= 0 || fragment.FontSize <= 0 {
			t.Fatalf("%s: the sample has no measured box: %+v", fragment.Style, fragment)
		}
	}
}

// ① shows every intro and outro preset by the renderer's own drawing, each slot
// holding the caller's label with its outline number (CLIP-165): every preset
// in design.json on every ratio, drawn with no scrim and ids of its own.
func TestEveryRegionPresetIsSampledWithItsSlotsNumbered(t *testing.T) {
	_, r := regionMeasured(t)
	if _, _, err := r.RegionPresetSamples(t.Context(), "vertical", "슬롯"); err != clip.ErrInvalid {
		t.Fatal("a label without {n} was drawn", err)
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		intro, outro, err := r.RegionPresetSamples(t.Context(), ratio, "슬롯 {n}")
		if err != nil {
			t.Fatal(ratio, err)
		}
		canvas, _ := clip.ClipCanvas(ratio)
		for kind, samples := range map[string][]clip.RegionPresetSample{"intro": intro, "outro": outro} {
			ids := design.RegionIDs(kind)
			if len(samples) != len(ids) {
				t.Fatal(ratio, kind, len(samples), ids)
			}
			for i, s := range samples {
				preset, _ := design.Region(kind, ids[i])
				if s.Preset != ids[i] {
					t.Fatal(ratio, kind, i, s.Preset)
				}
				for n := 1; n <= len(preset.Slots()); n++ {
					if !strings.Contains(s.SVG, fmt.Sprintf("슬롯 %d<", n)) {
						t.Fatalf("%s %s.%s does not number slot %d", ratio, kind, s.Preset, n)
					}
				}
				prefix := kind + "-" + s.Preset + "-"
				refs := strings.Count(s.SVG, `url(#`) + strings.Count(s.SVG, `href="#`)
				mine := strings.Count(s.SVG, `url(#`+prefix) + strings.Count(s.SVG, `href="#`+prefix)
				if strings.Count(s.SVG, `id="`) != strings.Count(s.SVG, `id="`+prefix) || refs != mine {
					t.Fatalf("%s %s.%s keeps a shared id", ratio, kind, s.Preset)
				}
				if strings.Contains(s.SVG, "scrim") || strings.Contains(s.SVG, "radialGradient") || !strings.HasPrefix(s.SVG, "<g>") {
					t.Fatalf("%s %s.%s drew a scrim or no group", ratio, kind, s.Preset)
				}
				if s.Box.Width <= 0 || s.Box.X < 0 || s.Box.Y < 0 || s.Box.X+s.Box.Width > float64(canvas.Width) || s.Box.Y+s.Box.Height > float64(canvas.Height) {
					t.Fatalf("%s %s.%s box %+v", ratio, kind, s.Preset, s.Box)
				}
			}
		}
	}
}
