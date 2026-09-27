package clip_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func storylineAnalyses() []clip.SourceAnalysis {
	return []clip.SourceAnalysis{
		{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "a"}}, Segments: []clip.Segment{{StartMS: 0, EndMS: 1000}, {StartMS: 1000, EndMS: 2000}}},
		{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "b"}}, Segments: []clip.Segment{{StartMS: 0, EndMS: 1000}}},
	}
}

// CLIP-178: a written storyline keeps its order, names only observed scenes, keeps a repeated
// scene where it was first named, and is held to its bounds.
func TestBoundStorylineKeepsOrderScenesAndBounds(t *testing.T) {
	observed := clip.ObservedScenes(storylineAnalyses())
	got := clip.BoundStoryline([]clip.StorylineParagraph{
		{Text: "  앞  ", ObservationIDs: []string{"a/0", "z/9", "a/0"}},
		{Text: "", ObservationIDs: []string{"z/1"}},
		{Text: "뒤", ObservationIDs: []string{"a/0", "b/0"}},
	}, observed)
	want := []clip.StorylineParagraph{{Text: "앞", ObservationIDs: []string{"a/0"}}, {Text: "뒤", ObservationIDs: []string{"b/0"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v, want %+v", got, want)
	}
	long := clip.BoundStoryline([]clip.StorylineParagraph{{Text: strings.Repeat("가", clip.StorylineTextMaxChars+1)}}, observed)
	if len([]rune(long[0].Text)) != clip.StorylineTextMaxChars {
		t.Fatal("a text past its bound was kept whole")
	}
}

// What changed since it was written: sources it was not made with, and scenes of the sources it
// was made with that no paragraph holds.
func TestAStorylineSaysWhatWasAddedAndTakenOut(t *testing.T) {
	s := clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "앞", ObservationIDs: []string{"a/0"}}}, MadeWithSources: []string{"a"}}
	if got := s.AddedSources([]string{"a", "b", "c"}); !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatal("added sources", got)
	}
	if got := s.TakenOutObservations(storylineAnalyses()); !reflect.DeepEqual(got, []string{"a/1"}) {
		t.Fatal("taken-out scenes", got)
	}
}

func TestAStorylineRoundTripsItsStoredForm(t *testing.T) {
	if raw, err := clip.EncodeStoryline(nil); err != nil || raw != "" {
		t.Fatal("none is not stored as none", raw, err)
	}
	if raw, err := clip.EncodeStoryline(&clip.Storyline{}); err != nil || raw != "" {
		t.Fatal("an empty storyline is not none", raw, err)
	}
	s := &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "앞", ObservationIDs: []string{"a/0"}}}, MadeWithSources: []string{"a"}}
	raw, err := clip.EncodeStoryline(s)
	if err != nil {
		t.Fatal(err)
	}
	back, err := clip.DecodeStoryline(raw)
	if err != nil || !reflect.DeepEqual(back, s) {
		t.Fatal("the storyline did not round-trip", back, err)
	}
	if none, err := clip.DecodeStoryline(""); none != nil || err != nil {
		t.Fatal("none did not read as none", none, err)
	}
}
