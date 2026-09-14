package clip_test

import (
	"math"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestCutCaptionSafeProjectsTheSourceAndTimeThroughCoverCrop(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	cut := clip.Cut{SourceID: "source", StartMS: 1000, EndMS: 2000, Focal: clip.Point{X: .5, Y: .5}}
	a := clip.SourceAnalysis{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Info: clip.MediaInfo{Width: 1920, Height: 1080}}}, Segments: []clip.Segment{{EndMS: 3000, CaptionSafe: []clip.Region{{X: .4, Y: .1, Width: .2, Height: .2}, {X: 0, Y: 0, Width: .1, Height: 1}}}, {StartMS: 3000, EndMS: 5000, CaptionSafe: []clip.Region{{X: 0, Y: 0, Width: 1, Height: 1}}}}}
	got := clip.CutCaptionSafe(canvas, cut, a)
	if len(got) != 1 {
		t.Fatalf("cropped-out/unrelated space retained: %+v", got)
	}
	want := clip.Region{X: 198.6666666666667, Y: 192, Width: 682.6666666666666, Height: 384}
	for _, delta := range []float64{got[0].X - want.X, got[0].Y - want.Y, got[0].Width - want.Width, got[0].Height - want.Height} {
		if math.Abs(delta) > 1e-9 {
			t.Fatalf("wrong cover projection: %+v", got)
		}
	}
	a.Source.ID = "other"
	if len(clip.CutCaptionSafe(canvas, cut, a)) != 0 {
		t.Fatal("space from another source used")
	}
	a.Source.ID = "source"
	cut.StartMS = 5000
	cut.EndMS = 6000
	if len(clip.CutCaptionSafe(canvas, cut, a)) != 0 {
		t.Fatal("space from another interval used")
	}
}

func TestCaptionSafeSpanningScenesRequiresCommonEmptySpace(t *testing.T) {
	canvas, _ := clip.ClipCanvas("square")
	cut := clip.Cut{SourceID: "source", StartMS: 5000, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}
	a := clip.SourceAnalysis{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Info: clip.MediaInfo{Width: 1080, Height: 1080}}}, Segments: []clip.Segment{{EndMS: 10000, CaptionSafe: []clip.Region{{Width: 1, Height: .3}}}, {StartMS: 10000, EndMS: 20000, CaptionSafe: []clip.Region{{Y: .1, Width: 1, Height: .3}}}}}
	got := clip.CutCaptionSafe(canvas, cut, a)
	if len(got) != 1 || got[0].X != 0 || got[0].Width != 1080 || math.Abs(got[0].Y-108) > 1e-9 || math.Abs(got[0].Height-216) > 1e-9 {
		t.Fatalf("invented union of safe scenes: %+v", got)
	}
	a.Segments[1].CaptionSafe = nil
	if len(clip.CutCaptionSafe(canvas, cut, a)) != 0 {
		t.Fatal("unobserved space treated as safe")
	}
}
