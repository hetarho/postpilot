package media

import (
	"fmt"
	"os"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// CDS-83 is a contract, not an intention: the fragment the editor is handed,
// placed at the box it is handed with it, rasterises to exactly the pixels the
// renderer draws for that caption — with the production resvg and the bundled
// faces, for a static style and for a sequence-rendered one.
func TestRenderSmokeCaptionFragmentMatchesTheRenderer(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 60000, Width: 1920, Height: 1080}}}
	for _, style := range []string{design.DefaultCaptionStyle, "neon"} {
		t.Run(style, func(t *testing.T) {
			plan := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: style})
			plan.CaptionStyles = []string{design.DefaultCaptionStyle, style}
			fragments, err := r.CaptionFragments(t.Context(), plan, sources)
			if err != nil {
				t.Fatal(err)
			}
			if len(fragments) != 1 {
				t.Fatalf("expected one caption, got %d", len(fragments))
			}
			f := fragments[0]
			if f.Sequence != (style != design.DefaultCaptionStyle) {
				t.Fatalf("%s was labelled the wrong rendering kind", style)
			}
			if err := a.WithWorkspace(t.Context(), "fragment-contract", func(ws clip.MediaWorkspace) error {
				canvas, _ := clip.ClipCanvas(plan.Ratio)
				layout, err := r.layoutComposition(t.Context(), ws, plan)
				if err != nil {
					return err
				}
				own, err := r.captionDocument(canvas, layout.visuals[0])
				if err != nil {
					return err
				}
				placed := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><g transform="translate(%s,%s)">%s</g></svg>`,
					canvas.Width, canvas.Height, coordinate(f.Box.X), coordinate(f.Box.Y), f.SVG)
				a, err := r.rasterize(t.Context(), ws, canvas, own, "renderer")
				if err != nil {
					return err
				}
				b, err := r.rasterize(t.Context(), ws, canvas, placed, "fragment")
				if err != nil {
					return err
				}
				left, err := os.ReadFile(a)
				if err != nil {
					return err
				}
				right, err := os.ReadFile(b)
				if err != nil {
					return err
				}
				// A pair of empty or failed rasterisations would agree about
				// nothing, so the comparison only means something once both
				// carry a drawn caption.
				if len(left) < 2000 {
					t.Fatalf("%s: the renderer drew %d bytes, which is not a caption", style, len(left))
				}
				if string(left) != string(right) {
					t.Fatalf("%s: the fragment drew %d bytes, the renderer %d — they must be the same pixels", style, len(right), len(left))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
