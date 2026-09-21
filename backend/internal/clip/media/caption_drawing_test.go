package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// The document a layer actually carries. The measured runner stands in for resvg
// by writing the SVG it was handed to the PNG it was asked for, so a layer's file
// IS the drawing that produced it.
func drawnLayer(t *testing.T, layer captionLayer) string {
	t.Helper()
	path := layer.Plate
	if layer.Sequence != nil {
		path = filepath.Join(layer.Sequence.Dir, "00001.png")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// CDS-4, CDS-85: the pace takes a caption's motion away and never its drawing.
// A rapid phrase in a plated or outlined style used to fall to the bundled
// template, which knows no style's plate and paints its raw fill — 스티커 and
// 버블 came out as near-black text and 아웃라인 emitted `fill=""`, which SVG
// resolves to black.
func TestEveryCaptionIsDrawnByItsOwnStyleAtEveryPace(t *testing.T) {
	for _, style := range design.CaptionStyles() {
		for _, pace := range []string{"steady", "rapid"} {
			t.Run(style.ID+"/"+pace, func(t *testing.T) {
				a, r, canvas, c, layout := captionLayout(t, style.ID)
				c.Pace = pace
				visual := declaredVisual{
					copy: c, caption: layout,
					manifest: clip.CompositionElement{Role: "caption", Style: style.ID, StartMS: 0, EndMS: 400,
						Parts: layout.Elements(0, 0, c, 0, 400)},
				}
				var drawn, own string
				if err := a.WithWorkspace(t.Context(), "caption-drawing", func(ws clip.MediaWorkspace) error {
					layer, err := r.declaredLayer(t.Context(), ws, canvas, &visual, clip.MediaSource{}, 0)
					if err != nil {
						return err
					}
					// A rapid phrase has nothing to animate, so it takes ONE
					// rasterisation whatever its style (CDS-81).
					if (layer.Sequence != nil) != (!style.Static() && pace != "rapid") {
						return fmt.Errorf("%s at %s drew a sequence: %v", style.ID, pace, layer.Sequence != nil)
					}
					drawn = drawnLayer(t, layer)
					own, err = r.captionDocument(canvas, visual)
					return err
				}); err != nil {
					t.Fatal(err)
				}
				// The drawing is the style's own, which is the drawing ② showed.
				if style.Static() != strings.Contains(drawn, `paint-order="stroke fill"`) {
					t.Fatalf("%s at %s was drawn by the other kind: %s", style.ID, pace, drawn)
				}
				if pace == "rapid" && drawn != own {
					t.Fatalf("%s at rapid pace is not the drawing its preview states: %s", style.ID, drawn)
				}
				// No caption ever renders with an empty fill: SVG resolves that
				// to black, which is how 아웃라인 lost its hollow letters.
				if strings.Contains(drawn, `fill=""`) {
					t.Fatalf("%s at %s emitted an empty fill: %s", style.ID, pace, drawn)
				}
				if style.Paint.Plate != "" && !strings.Contains(drawn, `fill="`+style.Paint.Plate+`"`) {
					t.Fatalf("%s at %s lost its plate: %s", style.ID, pace, drawn)
				}
				if style.Paint.Fill != "" && !strings.Contains(drawn, `fill="`+style.Paint.Fill+`"`) &&
					!strings.Contains(drawn, `fill="url(`) {
					t.Fatalf("%s at %s lost its ink: %s", style.ID, pace, drawn)
				}
				// What was drawn is recorded where CDS-52's V9 reads it.
				for _, part := range visual.manifest.Parts {
					if part.Kind == "copy" && part.Drawing != style.Rendering {
						t.Fatalf("%s at %s recorded the drawing %q", style.ID, pace, part.Drawing)
					}
				}
				if err := design.Verify(visual.manifest.Parts, "vertical", true); err != nil {
					t.Fatalf("%s at %s: %v", style.ID, pace, err)
				}
			})
		}
	}
}

// CDS-4: a rapid phrase replaces its neighbour with no fade and no movement
// whatever style it carries, so its one plate enters the chain unfaded and at a
// fixed y — the motion is what the pace takes away.
func TestARapidCaptionEntersTheChainWithoutFadeOrSettle(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	visuals := []declaredVisual{
		{text: clip.PortableText{Pace: "rapid"}, manifest: clip.CompositionElement{Role: "caption", Style: "sticker", StartMS: 1000, EndMS: 1400}},
	}
	graph := declaredOverlayGraph(cfg, overlayWindow{StartFrame: 0, EndFrame: 300, Layers: []int{0}}, visuals, []captionLayer{{Plate: "/w/declared-0000.png"}}, true)
	if strings.Contains(graph, "fade=t=") || !strings.Contains(graph, "overlay=x=0:y=0:") {
		t.Fatalf("a rapid phrase was faded or settled: %s", graph)
	}
	if !strings.Contains(graph, "loop=loop=299:size=1:start=0") {
		t.Fatalf("the one rasterisation was not held for the phrase's window: %s", graph)
	}
}

// CDS-85: the bundled template draws what a RULE states, which carries no
// style's plate and no style's ink, so a sequence style is refused by name
// rather than drawn as something else.
func TestTheBundledTemplateRefusesASequenceStyle(t *testing.T) {
	_, r, canvas, c, layout := captionLayout(t, "sticker")
	visual := declaredVisual{copy: c, caption: layout,
		manifest: clip.CompositionElement{Role: "caption", Style: "sticker", StartMS: 0, EndMS: 400}}
	if _, err := r.declaredSVG(canvas, visual); err == nil {
		t.Fatal("the template drew a style it cannot draw")
	}
}

// V9 fails a caption recorded as drawn by anything but its own style's drawing.
func TestVerificationRefusesACaptionDrawnAsAnotherKind(t *testing.T) {
	_, _, _, c, layout := captionLayout(t, "sticker")
	parts := layout.Elements(0, 0, c, 0, 400)
	for i := range parts {
		if parts[i].Kind == "copy" {
			parts[i].Drawing = design.StaticCaption
		}
	}
	err := design.Verify(parts, "vertical", true)
	if err == nil || !strings.Contains(err.Error(), string(design.ViolationMotion)) {
		t.Fatalf("a caption drawn by the wrong drawing passed verification: %v", err)
	}
}
