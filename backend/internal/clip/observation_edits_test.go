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

// CLIP-235: a corrected analysis is kept whole as recorded, whichever contract recorded it,
// including a legacy scene with no status crossing today's chunk boundary; an uncorrected or
// malformed one is not.
func TestValidatePinnedAnalysis(t *testing.T) {
	l, analyses := editFixture()
	pinned := analyses[0]
	pinned.OwnerEdited = true
	if err := ValidatePinnedAnalysis(l, pinned); err != nil {
		t.Fatal("a corrected v2 analysis was refused", err)
	}
	legacy := pinned
	legacy.Segments = []Segment{{StartMS: 0, EndMS: 90000, Event: "corrected legacy scene", Quality: "clear", Focal: Point{X: .5, Y: .5}, OwnerEdited: true, OwnerFields: []string{OwnerFieldEvent}}}
	if err := ValidatePinnedAnalysis(l, legacy); err != nil {
		t.Fatal("a corrected legacy range crossing a chunk boundary was refused", err)
	}
	scenes := func(step int) SourceAnalysis {
		a := pinned
		a.Segments = nil
		for at := 0; at < 90000; at += step {
			a.Segments = append(a.Segments, Segment{StartMS: at, EndMS: at + step, Event: "scene", Quality: "clear", Focal: Point{X: .5, Y: .5}, Certainty: CertaintyCertain, Usability: UsabilityUsable})
		}
		return a
	}
	if err := ValidatePinnedAnalysis(l, scenes(10000)); err != nil {
		t.Fatal("the scenes two chunks admit together were refused", err)
	}
	uncorrected := legacy
	uncorrected.OwnerEdited = false
	overlapping := legacy
	overlapping.Segments = []Segment{legacy.Segments[0], legacy.Segments[0]}
	outside := legacy
	outside.Segments = []Segment{legacy.Segments[0]}
	outside.Segments[0].EndMS = 91000
	invented := legacy
	invented.Segments = []Segment{legacy.Segments[0]}
	invented.Segments[0].Certainty = "probably"
	foreign := legacy
	foreign.Source.Fingerprint = ""
	for name, a := range map[string]SourceAnalysis{"uncorrected": uncorrected, "overlapping": overlapping, "outside": outside, "invented status": invented, "no fingerprint": foreign, "more scenes than two chunks admit": scenes(5000)} {
		if ValidatePinnedAnalysis(l, a) == nil {
			t.Fatal("a pinned analysis was accepted:", name)
		}
	}
}
