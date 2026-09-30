package rpc

import (
	"testing"

	"github.com/postpilot/backend/internal/experiment"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice"
)

// MODEL-67, MODEL-32: a voice-sourced comparison reads its source, prompt and the owner's answer,
// and each candidate's output is its piece with its comparison — still blind before a verdict.
func TestAVoiceSourcedComparisonOnTheWire(t *testing.T) {
	found := experiment.Experiment{
		ID: "exp", Stage: experiment.StageWrite, Origin: experiment.OriginLab, Source: experiment.SourceVoice,
		Status: experiment.StatusReview, VoiceID: "voice-a", VoicePromptKey: "opening_greeting",
		Candidates: []experiment.Candidate{
			{ID: "left", DisplaySide: experiment.SideLeft, Status: experiment.CandidateSucceeded, Output: []byte("안녕하세요!"), Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}},
			{ID: "right", DisplaySide: experiment.SideRight, Status: experiment.CandidateFailed, Model: experiment.ModelRef{ProviderID: "p", ModelID: "b"}},
		},
	}
	detail := experiment.ReflectionDetail{PromptText: "첫인사를 써 보세요.", Answer: "안녕하세요, 동네 빵집이에요.", Comparisons: map[string][]experiment.ItemComparison{
		"left": {{Item: "endings", Distance: 0.5, Headline: "해요", Facets: []experiment.ComparisonFacet{
			{Key: "해요", Unit: "share", Voice: 0.9, Text: 0.4},
			{Key: "suffixes", Unit: "text", VoiceTerms: []string{"더라구요"}, TextTerms: []string{}},
		}}},
	}}
	mapped := toProtoExperiment(found, detail, false)
	if mapped.GetSource() != postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_VOICE || mapped.GetVoicePromptKey() != "opening_greeting" ||
		mapped.GetVoicePromptText() != "첫인사를 써 보세요." || mapped.GetVoiceAnswer() != "안녕하세요, 동네 빵집이에요." {
		t.Fatalf("experiment = %+v", mapped)
	}
	left := mapped.GetCandidates()[0]
	piece := left.GetVoicePiece()
	if piece.GetText() != "안녕하세요!" || len(piece.GetComparison()) != 1 || left.GetModel() != nil {
		t.Fatalf("left = %+v", left)
	}
	item := piece.GetComparison()[0]
	if item.GetItem() != postpilotv1.FingerprintItem_FINGERPRINT_ITEM_ENDINGS || item.GetHeadline() != "해요" ||
		item.GetFacets()[0].GetUnit() != postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_SHARE || item.GetFacets()[0].GetText().GetNumber() != 0.4 ||
		item.GetFacets()[1].GetVoice().GetTerms().GetTerms()[0] != "더라구요" {
		t.Fatalf("comparison = %+v", item)
	}
	if mapped.GetCandidates()[1].GetOutput() != nil {
		t.Fatal("a failed candidate carried a piece")
	}
	post := toProtoExperiment(experiment.Experiment{Stage: experiment.StageWrite, Source: experiment.SourcePost}, experiment.ReflectionDetail{}, false)
	if post.GetSource() != postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_POST {
		t.Fatalf("a post comparison's source = %v", post.GetSource())
	}
	for value, want := range map[postpilotv1.ExperimentSource]experiment.Source{
		postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_UNSPECIFIED: "",
		postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_POST:        experiment.SourcePost,
		postpilotv1.ExperimentSource_EXPERIMENT_SOURCE_VOICE:       experiment.SourceVoice,
	} {
		if got := fromProtoSource(value); got != want {
			t.Fatalf("source %v = %q", value, got)
		}
	}
}

// ARCH-3: this edge names every counted item and facet unit the voice context has, each to its
// own generated value, and no generated value is left without one.
func TestTheReflectionEdgeKnowsEveryItemAndUnit(t *testing.T) {
	seen := map[postpilotv1.FingerprintItem]bool{}
	for _, item := range voice.Items() {
		mapped, ok := protoItems[string(item)]
		if !ok || seen[mapped] {
			t.Fatalf("%q maps to %v", item, mapped)
		}
		seen[mapped] = true
	}
	if len(seen) != len(postpilotv1.FingerprintItem_name)-1 {
		t.Fatalf("mapped %d of %d items", len(seen), len(postpilotv1.FingerprintItem_name)-1)
	}
	units := map[postpilotv1.FingerprintFacetUnit]bool{}
	for _, unit := range voice.FacetUnits() {
		mapped, ok := protoUnits[string(unit)]
		if !ok || units[mapped] {
			t.Fatalf("%q maps to %v", unit, mapped)
		}
		units[mapped] = true
	}
	if len(units) != len(postpilotv1.FingerprintFacetUnit_name)-1 {
		t.Fatalf("mapped %d of %d units", len(units), len(postpilotv1.FingerprintFacetUnit_name)-1)
	}
}
