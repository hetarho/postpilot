package clip_test

import (
	"math"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

func analysisSource(id string, duration int) clip.AnalysisSource {
	return clip.AnalysisSource{RenderSource: clip.RenderSource{ID: id, Fingerprint: "fingerprint-" + id, Info: clip.MediaInfo{DurationMS: duration, Width: 1080, Height: 1920, HasAudio: true}}, Filename: id + ".mp4"}
}
func analysisSegment(start, end int) clip.Segment {
	return clip.Segment{StartMS: start, EndMS: end, Event: "음식을 접시에 담는다", Quality: "stable, in focus", Focal: clip.Point{X: .5, Y: .5}, Subject: clip.Region{X: .2, Y: .6, Width: .6, Height: .3}}
}
func TestMergeAnalysesPinsCompleteManifestAndAbsoluteTime(t *testing.T) {
	l := config.ClipAI(&config.Config{}).Analysis
	sources := []clip.AnalysisSource{analysisSource("one", 65000), analysisSource("two", 1000)}
	chunks := []clip.ChunkAnalysis{
		{SourceID: "one", Fingerprint: "fingerprint-one", Index: 0, DurationMS: 60000, Segments: []clip.Segment{analysisSegment(0, 60000)}},
		{SourceID: "one", Fingerprint: "fingerprint-one", Index: 1, OffsetMS: 60000, DurationMS: 5000, Segments: []clip.Segment{analysisSegment(60000, 65000)}},
		{SourceID: "two", Fingerprint: "fingerprint-two", DurationMS: 1000, Segments: []clip.Segment{analysisSegment(0, 1000)}},
	}
	got, err := clip.MergeAnalyses(l, sources, chunks)
	if err != nil || len(got) != 2 || got[0].Segments[1].StartMS != 60000 {
		t.Fatalf("%+v %v", got, err)
	}
	for _, mode := range []string{"duplicate source", "duplicate fingerprint", "out of order", "missing chunk", "extra chunk", "gap", "wrong fingerprint", "outside chunk", "overlap", "empty description", "nan", "overflow index"} {
		t.Run(mode, func(t *testing.T) {
			ss := append([]clip.AnalysisSource(nil), sources...)
			cc := append([]clip.ChunkAnalysis(nil), chunks...)
			for i := range cc {
				cc[i].Segments = append([]clip.Segment(nil), cc[i].Segments...)
			}
			switch mode {
			case "duplicate source":
				ss[1].ID = ss[0].ID
			case "duplicate fingerprint":
				ss[1].Fingerprint = ss[0].Fingerprint
			case "out of order":
				cc[0], cc[2] = cc[2], cc[0]
			case "missing chunk":
				cc = cc[:2]
			case "extra chunk":
				cc = append(cc, chunks[2])
			case "gap":
				cc[1].OffsetMS++
			case "wrong fingerprint":
				cc[0].Fingerprint = "other"
			case "outside chunk":
				cc[0].Segments[0].EndMS++
			case "overlap":
				cc[0].Segments = append(cc[0].Segments, analysisSegment(10, 20))
			case "empty description":
				cc[0].Segments[0].Event = "  "
			case "nan":
				cc[0].Segments[0].Focal.X = math.NaN()
			case "overflow index":
				cc[0].Index = math.MaxInt
			}
			if _, err := clip.MergeAnalyses(l, ss, cc); err == nil {
				t.Fatal("invalid merge accepted")
			}
		})
	}
	if err := clip.ValidateChunkInput(l, clip.ChunkInput{Source: sources[0], Index: math.MaxInt, DurationMS: 60000}); err == nil {
		t.Fatal("overflow index")
	}
}
func TestCaptionAvoidFollowsCoverCrop(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	a := clip.SourceAnalysis{Source: analysisSource("one", 15000), Segments: []clip.Segment{analysisSegment(0, 15000)}}
	cut := clip.Cut{StartMS: 0, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}
	r := clip.CaptionAvoid(canvas, cut, a)
	if math.Abs(r.X-.2) > 1e-9 || math.Abs(r.Y-.6) > 1e-9 || math.Abs(r.Width-.6) > 1e-9 {
		t.Fatal(r)
	}
	a.Source.Info.Width = 1920
	a.Source.Info.Height = 1080
	a.Segments[0].Subject = clip.Region{Width: .1, Height: 1}
	if got := clip.CaptionAvoid(canvas, cut, a); got != (clip.Region{}) {
		t.Fatalf("cropped-out subject remains: %+v", got)
	}
	a.Segments[0].StartMS = 15000
	a.Segments[0].EndMS = 16000
	if got := clip.CaptionAvoid(canvas, cut, a); got != (clip.Region{}) {
		t.Fatal("unrelated time")
	}
}
