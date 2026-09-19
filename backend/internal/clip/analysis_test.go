package clip_test

import (
	"math"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
)

func analysisSource(id string, duration int) clip.AnalysisSource {
	return clip.AnalysisSource{RenderSource: clip.RenderSource{ID: id, Fingerprint: "fingerprint-" + id, Info: clip.MediaInfo{DurationMS: duration, Width: 1080, Height: 1920, HasAudio: true}}, Filename: id + ".mp4"}
}
func analysisSegment(start, end int) clip.Segment {
	return clip.Segment{StartMS: start, EndMS: end, Event: "음식을 접시에 담는다", Action: "젓가락으로 집는다", Motion: "static", Quality: "stable, in focus", Focal: clip.Point{X: .5, Y: .5}, Subject: clip.Region{X: .2, Y: .6, Width: .6, Height: .3}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}
}
func TestMergeAnalysesPinsCompleteManifestAndAbsoluteTime(t *testing.T) {
	l := ai.DefaultConfig(clip.Environment{}).Analysis
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
	for _, mode := range []string{"duplicate source", "duplicate fingerprint", "out of order", "missing chunk", "extra chunk", "gap", "wrong fingerprint", "outside chunk", "overlap", "empty description", "nan", "overflow index", "missing start", "internal gap", "missing end", "no status", "invalid status"} {
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
				cc[0].Segments[0].Event, cc[0].Segments[0].Action, cc[0].Segments[0].Motion = "  ", "", ""
			case "nan":
				cc[0].Segments[0].Focal.X = math.NaN()
			case "overflow index":
				cc[0].Index = math.MaxInt
			case "missing start":
				cc[0].Segments[0].StartMS = 1
			case "internal gap":
				cc[0].Segments[0].EndMS = 30000
				cc[0].Segments = append(cc[0].Segments, analysisSegment(30001, 60000))
			case "missing end":
				cc[0].Segments[0].EndMS = 59999
			case "no status":
				cc[0].Segments[0].Certainty, cc[0].Segments[0].Usability = "", ""
			case "invalid status":
				cc[0].Segments[0].Usability = "maybe"
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

// Black, obscured and unknowable footage is RECORDED with its exact span; only
// its quality reason carries meaning, and a v2 chunk still has to be complete.
func TestUnknowableSpansAreRecordedWithoutInventedDescription(t *testing.T) {
	l := ai.DefaultConfig(clip.Environment{}).Analysis
	unknown := analysisSegment(0, 5000)
	unknown.Event, unknown.Action, unknown.Motion, unknown.Subjects = "", "", "", nil
	unknown.Certainty, unknown.Usability = clip.CertaintyUnknown, clip.UsabilityUnusable
	unknown.Quality = "완전히 어두워 아무것도 식별할 수 없음"
	if err := clip.ValidateSegments(l, []clip.Segment{unknown, analysisSegment(5000, 10000)}, 0, 10000); err != nil {
		t.Fatalf("an unknowable span must be recordable: %v", err)
	}
	blank := unknown
	blank.Quality = "   "
	if err := clip.ValidateSegments(l, []clip.Segment{blank, analysisSegment(5000, 10000)}, 0, 10000); err == nil {
		t.Fatal("an unknowable span still owes a reason")
	}
	described := analysisSegment(0, 5000)
	described.Event, described.Action, described.Motion, described.Subjects = "", "", "", nil
	if err := clip.ValidateSegments(l, []clip.Segment{described, analysisSegment(5000, 10000)}, 0, 10000); err == nil {
		t.Fatal("a certain span with nothing described was accepted")
	}
	// A v1 record keeps the rules it was written under: gapped, status-less and
	// still readable, but never promoted into the v2 contract.
	legacy := analysisSegment(0, 4000)
	legacy.Certainty, legacy.Usability = "", ""
	if err := clip.ValidateLegacySegments(l, []clip.Segment{legacy}, 0, 10000); err != nil {
		t.Fatalf("a stored v1 record must stay readable: %v", err)
	}
	if err := clip.ValidateSegments(l, []clip.Segment{legacy}, 0, 10000); err == nil {
		t.Fatal("a v1 record passed the v2 contract")
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
