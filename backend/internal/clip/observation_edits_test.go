package clip

import (
	"errors"
	"slices"
	"testing"
)

func editedText(v string) *string { return &v }

func editFixture() (AnalysisLimits, []SourceAnalysis) {
	l := AnalysisLimits{ChunkMS: 60000, MaxSources: 4, MaxSourceDurationMS: 600000, MaxSegments: 8, MaxTextRunes: 40, MaxSubjects: 3}
	source := AnalysisSource{RenderSource: RenderSource{ID: "s", Fingerprint: "f", Info: MediaInfo{DurationMS: 90000, Width: 1920, Height: 1080}}, Filename: "s.mp4"}
	scene := func(start, end int, speech string) Segment {
		return Segment{StartMS: start, EndMS: end, Event: "scene", Speech: speech, Quality: "clear", Focal: Point{X: .5, Y: .5}, Certainty: CertaintyCertain, Usability: UsabilityUsable}
	}
	return l, []SourceAnalysis{{Source: source, Segments: []Segment{scene(0, 30000, ""), scene(30000, 60000, "hello"), scene(60000, 90000, "")}}}
}

// CLIP-235: corrections apply to a copy, record the changed fields, and refuse a change of
// audibility or a scene the analysis lacks.
func TestApplyObservationEdits(t *testing.T) {
	l, analyses := editFixture()
	out, err := ApplyObservationEdits(l, analyses, []ObservationEdit{{ObservationID: "s/1", Speech: editedText("hello there"), Event: editedText("scene")}})
	if err != nil {
		t.Fatal(err)
	}
	if analyses[0].Segments[1].Speech != "hello" || analyses[0].OwnerEdited {
		t.Fatal("the input was changed")
	}
	if got := out[0].Segments[1]; got.Speech != "hello there" || !slices.Equal(got.OwnerFields, []string{"speech"}) || !got.OwnerEdited || !out[0].OwnerEdited {
		t.Fatalf("an unchanged value was recorded or the change lost: %+v", got)
	}
	unchanged, err := ApplyObservationEdits(l, analyses, []ObservationEdit{{ObservationID: "s/0", Event: editedText("scene")}})
	if err != nil || unchanged[0].OwnerEdited {
		t.Fatal("an edit to the recorded value marked the source", err)
	}
	for id, edit := range map[string]ObservationEdit{
		"speech_audibility":   {ObservationID: "s/1", Speech: editedText("")},
		"observation_unknown": {ObservationID: "s/3", Event: editedText("x")},
		"observe_description": {ObservationID: "s/0", Event: editedText("")},
	} {
		var refused *ObservationEditError
		if _, err := ApplyObservationEdits(l, analyses, []ObservationEdit{edit}); !errors.As(err, &refused) || refused.Check != id {
			t.Fatal(id, err)
		}
	}
	var refused *ObservationEditError
	if _, err := ApplyObservationEdits(l, analyses, []ObservationEdit{{ObservationID: "s/0", Event: editedText("a")}, {ObservationID: "s/0", Event: editedText("b")}}); !errors.As(err, &refused) || refused.Check != "observation_repeated" {
		t.Fatal("a repeated id was accepted", err)
	}
	for _, id := range []string{"s", "s/", "s/-1", "s/01", "/0"} {
		if _, _, ok := ObservationIndex(id); ok {
			t.Fatal("malformed id parsed", id)
		}
	}
}

// SplitAnalysis is MergeAnalyses' inverse, so a corrected analysis re-enters a run as chunks.
func TestSplitAnalysisInvertsMerge(t *testing.T) {
	l, analyses := editFixture()
	analyses[0].Segments[2].OwnerEdited = true
	chunks, err := SplitAnalysis(l, analyses[0])
	if err != nil || len(chunks) != 2 || len(chunks[0].Segments) != 2 || chunks[1].OffsetMS != 60000 || chunks[1].DurationMS != 30000 {
		t.Fatalf("%v %+v", err, chunks)
	}
	merged, err := MergeAnalyses(l, []AnalysisSource{analyses[0].Source}, chunks)
	if err != nil || !merged[0].OwnerEdited || len(merged[0].Segments) != 3 {
		t.Fatal("the merge of the split is not the analysis", err)
	}
	crossing := analyses[0]
	crossing.Segments = []Segment{crossing.Segments[0], {StartMS: 30000, EndMS: 90000, Event: "long", Quality: "clear", Certainty: CertaintyCertain, Usability: UsabilityUsable, Focal: Point{X: .5, Y: .5}}}
	if _, err := SplitAnalysis(l, crossing); err == nil {
		t.Fatal("a scene crossing a chunk boundary was split")
	}
}
