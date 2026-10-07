package media

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestGroundedSequencePaintAndManifestUseTheSameBackdrop(t *testing.T) {
	for _, style := range []string{"word-pop", "outline", "serif"} {
		for _, bright := range []bool{false, true} {
			t.Run(style+map[bool]string{false: " dark", true: " bright"}[bright], func(t *testing.T) {
				plan := declaredPlan(t, `<clip version="1" styles="`+style+`"><text id="caption" kind="fixed" role="caption" style="`+style+`" position="bottom" basis="output-start" start="0" end="4">지금 보는 화면 기록</text></clip>`, "vertical")
				plan.CaptionStyles = []string{style}
				plan.Accent = "blue"
				layout := measuredDeclared(t, plan)
				visual := layout.visuals[0]
				canvas, _ := clip.ClipCanvas("vertical")
				level := 0.0
				if bright {
					level = 1
				}
				visual.ground = Luminance{Mean: level, R: level, G: level, B: level, Frames: []float64{level, level, level}}
				applyDeclaredGround(canvas, &visual)
				frame := captionFrame(canvas, visual.copy, visual.caption, 4000, 0.5)
				defs, body, ok := design.DrawCaptionFrame(frame)
				if !ok {
					t.Fatal("sequence style missing")
				}
				if bright {
					if frame.GroundScrim == nil || !strings.Contains(defs, "groundScrim") || !strings.Contains(body, "url(#groundScrim)") {
						t.Fatal("manifest credited an absent drawn scrim", defs, body)
					}
					if visual.caption.Caption.Paint.Accent && frame.Accent != design.Color["text_white"].Hex {
						t.Fatal("bright sequence kept colored accent", frame.Accent)
					}
					crop := sequenceCrop(canvas, visual.caption)
					band := frame.GroundScrim.Region
					if crop.X > band.X || crop.Y > band.Y || crop.X+crop.Width < band.X+band.Width || crop.Y+crop.Height < band.Y+band.Height {
						t.Fatal("native crop clipped scrim", crop, band)
					}
				} else if frame.GroundScrim != nil || strings.Contains(defs, "groundScrim") {
					t.Fatal("dark sequence gained scrim")
				}
				manifestScrim := false
				for _, p := range visual.manifest.Parts {
					if p.Kind == "scrim" {
						manifestScrim = true
					}
				}
				if manifestScrim != bright {
					t.Fatal("draw/manifest scrim condition differs", visual.manifest.Parts)
				}
			})
		}
	}
}

func TestPlatedCaptionsNeverReadGroundOrReplaceTheirOwnPaint(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1" styles="bubble"><text id="caption" kind="fixed" role="caption" style="bubble" position="bottom" basis="output-start" start="0" end="4">화면 기록</text></clip>`, "vertical")
	plan.CaptionStyles = []string{"bubble"}
	layout := measuredDeclared(t, plan)
	if !layout.visuals[0].caption.plated() {
		t.Fatal("fixture lost plate")
	}
	canvas, _ := clip.ClipCanvas("vertical")
	r := Rendering{cfg: clip.RenderConfig{FPS: 30}}
	if got := r.newGroundSampler(canvas, newCutTimeline(30, plan), layout.visuals); len(got.reads) != 0 {
		t.Fatal("plated caption was sampled")
	}
	visual := layout.visuals[0]
	visual.ground = Luminance{Mean: 1, Frames: []float64{1}}
	applyDeclaredGround(canvas, &visual)
	if visual.caption.GroundScrim != nil || visual.caption.Ground.Sampled() {
		t.Fatal("plate paint replaced by footage decisions")
	}
}
