package media

import (
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
