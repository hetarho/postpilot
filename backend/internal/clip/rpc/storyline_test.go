package rpc

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// CLIP-178: the project carries its storyline with what changed since it was written — the
// sources added and the observed scenes of the made-with sources no paragraph holds — and none
// when it has none.
func TestTheProjectCarriesItsStorylineAndWhatChangedSince(t *testing.T) {
	analyses := []clip.SourceAnalysis{
		{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "a"}}, Segments: []clip.Segment{{EndMS: 1000}, {StartMS: 1000, EndMS: 2000}}},
		{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "b"}}, Segments: []clip.Segment{{EndMS: 1000}}},
	}
	raw, err := json.Marshal(analyses)
	if err != nil {
		t.Fatal(err)
	}
	p := clip.Project{ID: "clip", Analysis: string(raw), Storyline: &clip.Storyline{
		Paragraphs:      []clip.StorylineParagraph{{Text: "앞", ObservationIDs: []string{"a/0"}}, {Text: "뒤"}},
		EditedByHand:    true,
		MadeWithSources: []string{"a"},
	}}
	out := projectProto(p).GetStoryline()
	if out == nil || len(out.Paragraphs) != 2 || out.Paragraphs[0].Text != "앞" || !reflect.DeepEqual(out.Paragraphs[0].ObservationIds, []string{"a/0"}) || !out.EditedByHand {
		t.Fatalf("the storyline was not carried: %+v", out)
	}
	// Without the source read, the analysed sources stand for the current ones.
	if !reflect.DeepEqual(out.AddedSourceIds, []string{"b"}) || !reflect.DeepEqual(out.TakenOutObservationIds, []string{"a/1"}) {
		t.Fatalf("added %v, taken out %v", out.AddedSourceIds, out.TakenOutObservationIds)
	}
	// With it, a source uploaded since reads as added before any generation analysed it.
	current := currentSourceIDs([]clip.SourceBatch{
		{Current: false, Sources: []clip.SourceLease{{ID: "old"}}},
		{Current: true, Sources: []clip.SourceLease{{ID: "a"}, {ID: "c"}}},
	})
	if got := storylineProto(p, current).AddedSourceIds; !reflect.DeepEqual(got, []string{"c"}) {
		t.Fatal("added against the current batch", got)
	}
	p.Storyline = nil
	if projectProto(p).GetStoryline() != nil {
		t.Fatal("a clip with no storyline carried one")
	}
}
