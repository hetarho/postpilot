package media

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Optional actual-browser evidence is consumed by the independent native
// sampling timeline/math. The input RGB frames were decoded by real FFmpeg,
// while each observed ROI was drawn by the browser's production original path.
func TestBrowserOriginalBackgroundProof(t *testing.T) {
	path := os.Getenv("POSTPILOT_BROWSER_BACKGROUND_PROOF")
	if path == "" {
		t.Skip("actual browser report not supplied")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var report struct {
		Qualification bool
		Results       []struct {
			ID           string
			NativeFrames [][3]uint8
		}
		Backgrounds []struct {
			IDs          []string
			Ratio        string
			TransitionMS int
			Evidence     struct {
				Notices      []struct{ Code, Action, ElementID, CutID string }
				Measurements []struct {
					ElementID string
					Geometry  struct {
						Region        design.Bounds
						Anchor        string
						ContrastParts []struct {
							Box    design.Bounds
							Fill   string
							Alpha  float64
							Stroke bool
						}
					}
					SampleFrames []int
					Ground       struct {
						Mean, Sigma float64
						Frames      []float64
					}
					Scrim, AccentWhite bool
				}
			}
		}
	}
	if e = json.Unmarshal(raw, &report); e != nil {
		t.Fatal(e)
	}
	if report.Qualification || len(report.Backgrounds) < 24 {
		t.Fatal("missing bounded actual comparison evidence")
	}
	sources := map[string][][3]uint8{}
	for _, r := range report.Results {
		if len(r.NativeFrames) != 30 {
			t.Fatal("incomplete native reference", r.ID)
		}
		sources[r.ID] = r.NativeFrames
	}
	r := &Rendering{cfg: clip.RenderConfig{FPS: 30}}
	for _, bc := range report.Backgrounds {
		plan := clip.EditPlan{}
		for i := range bc.IDs {
			cut := clip.Cut{StartMS: 0, EndMS: 1000}
			if i > 0 {
				cut.TransitionMS = bc.TransitionMS
			}
			plan.Cuts = append(plan.Cuts, cut)
		}
		timeline := newCutTimeline(30, plan)
		start, end := 0, 1000
		if len(bc.IDs) > 1 {
			start, end = 1000-bc.TransitionMS, 1000
		}
		canvas, e := clip.ClipCanvas(bc.Ratio)
		if e != nil {
			t.Fatal(e)
		}
		sampler := r.newGroundSampler(canvas, timeline, []declaredVisual{{manifest: clip.CompositionElement{Role: "caption", StartMS: start, EndMS: end, Parts: design.Manifest{{Kind: "copy", Region: design.Bounds{Width: 1, Height: 1}}}}}})
		frames := []int{}
		window := declaredSampleWindow(start, end, timeline.total*1000/30, 30)
		for _, at := range sampleOffsets(clip.Cut{EndMS: timeline.total * 1000 / 30}, window) {
			frames = append(frames, min(timeline.total-1, (at*30+999)/1000))
		}
		for cut, id := range bc.IDs {
			for _, frame := range sampler.needs(cut) {
				rgb := sources[id][frame]
				img := image.NewRGBA(image.Rect(0, 0, 1, 1))
				img.SetRGBA(0, 0, color.RGBA{rgb[0], rgb[1], rgb[2], 255})
				sampler.take(cut, frame, img)
			}
		}
		grounds, e := sampler.grounds()
		if e != nil {
			t.Fatal(e)
		}
		want := grounds[0]
		if len(bc.Evidence.Measurements) != 4 {
			t.Fatal("missing role coverage", bc.Ratio, bc.IDs)
		}
		for _, got := range bc.Evidence.Measurements {
			if !reflect.DeepEqual(frames, got.SampleFrames) || len(got.Ground.Frames) != 3 || math.Abs(want.Mean-got.Ground.Mean) > 1e-7 || math.Abs(want.Sigma-got.Ground.Sigma) > 1e-7 || want.Scrim() != got.Scrim || (want.Mean >= design.Luma.ScrimThreshold) != got.AccentWhite {
				t.Fatalf("%s %v transition=%d role=%s: native=%+v observed=%+v frames=%v", bc.Ratio, bc.IDs, bc.TransitionMS, got.ElementID, want, got, frames)
			}
			visual := declaredVisual{ground: want}
			if got.ElementID == "hook" || got.ElementID == "ending" {
				kind, id, rows := "intro", "b", []string{"시작", "기록"}
				if got.ElementID == "ending" {
					kind, id, rows = "outro", "e", []string{"평점", "4.5", "다음 기록"}
				}
				layout, e := design.LayoutRegion(kind, id, bc.Ratio, rows)
				if e != nil {
					t.Fatal(e)
				}
				visual.block = newRegionBlock(canvas, kind, id, layout)
				for i, part := range got.Geometry.ContrastParts {
					fill, alpha := layout.Slots[i].Spec.Paint()
					if fill != part.Fill || alpha != part.Alpha {
						t.Fatal("regional paint drift", part, fill, alpha)
					}
					visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "copy", Slot: i + 1, Region: part.Box, Fill: fill, Opacity: alpha})
				}
				applyRegionGround(canvas, &visual)
			} else {
				for i, part := range got.Geometry.ContrastParts {
					anchor := got.Geometry.Anchor
					if anchor == "header" || anchor == "auto" {
						anchor = "top"
						if part.Box.Y+part.Box.Height/2 > float64(canvas.Height)/2 {
							anchor = "bottom"
						}
					}
					fill, alpha := design.Color["text_white"].Hex, 1.0
					if got.ElementID == "info" && i == 0 {
						fill, alpha = design.Color["text_muted"].Hex, design.Color["text_muted"].Alpha
					}
					if fill != part.Fill || alpha != part.Alpha || !part.Stroke {
						t.Fatal("caption/info paint drift", part)
					}
					background := want.Background(canvas, design.StyleRule{Stroke: "text"}, anchor, clip.Region(part.Box))
					visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "copy", Region: part.Box, Fill: fill, Opacity: alpha, Background: background})
				}
			}
			gotNotice := false
			for _, n := range bc.Evidence.Notices {
				if n.ElementID == got.ElementID && n.Code == "composition_contrast" && n.Action == "shortfall" {
					gotNotice = true
				}
			}
			if gotNotice == design.Legible(visual.manifest.Parts) {
				t.Fatal("native contrast notice drift", bc.Ratio, bc.IDs, got.ElementID, gotNotice, visual.manifest.Parts)
			}
		}
	}
}
