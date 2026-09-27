package clip_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// The scene and the readable-text flag come from the segments a cut spans, and
// the subject box arrives in canvas pixels for CDS-38's 15 % rule.
func TestCutSceneAndSubjectProjection(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	analysis := clip.SourceAnalysis{
		Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "s", Fingerprint: "f", Info: clip.MediaInfo{DurationMS: 20000, Width: 1080, Height: 1920}}},
		Segments: []clip.Segment{
			{StartMS: 0, EndMS: 5000, Scene: "menu", ReadableText: true, Subject: clip.Region{X: .25, Y: .5, Width: .5, Height: .25}},
			{StartMS: 5000, EndMS: 20000, Scene: "food"},
		},
	}
	cut := clip.Cut{StartMS: 0, EndMS: 4000, Focal: clip.Point{X: .5, Y: .5}}
	scene, readable := clip.CutScene(cut, analysis)
	if scene != "menu" || !readable {
		t.Fatalf("scene=%s readable=%v", scene, readable)
	}
	subject := clip.CutSubject(canvas, cut, analysis)
	if subject.X != 270 || subject.Width != 540 || subject.Y != 960 || subject.Height != 480 {
		t.Fatalf("subject in canvas pixels: %+v", subject)
	}
	// A stored analysis written before scenes existed reads as the default row
	// with no subject box, never as an unknown scene.
	legacy := clip.SourceAnalysis{Source: analysis.Source, Segments: []clip.Segment{{StartMS: 0, EndMS: 20000}}}
	scene, readable = clip.CutScene(cut, legacy)
	if scene != design.DefaultScene || readable {
		t.Fatalf("legacy scene=%s readable=%v", scene, readable)
	}
	if got := clip.CutSubject(canvas, cut, legacy); got != (clip.Region{}) {
		t.Fatalf("legacy subject %+v", got)
	}
}
