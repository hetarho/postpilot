package ai_test

import (
	"encoding/json"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/composition"
)

// A caption entry's declared maximum reaches the narration call beside the
// entry it bounds; an entry that declares none carries none (CLIP-116).
func TestWriterIsToldEachDeclaredTextMaximum(t *testing.T) {
	in := nativeInput()
	setNativeBody(&in, `<clip version="1"><text id="line" kind="ai" role="caption" chars="7">관찰</text><text id="free" kind="ai" role="caption">관찰</text></clip>`)
	in.Composition.Inputs = clip.CompositionInputs{Values: map[string]string{}, Items: map[string][]composition.Item{}}
	flow := clip.EditPlan{Ratio: in.Ratio, DurationMS: 15000, Cuts: []clip.Cut{{ID: "whole", SourceID: "source", Fingerprint: in.Analyses[0].Source.Fingerprint, EndMS: 15000, PlaybackRatePermille: clip.RateUnitPermille}},
		Portable: &clip.PortablePlan{Cuts: []composition.Cut{{ID: "whole", SourceID: "source", EndMS: 15000, PlaybackRatePermille: clip.RateUnitPermille}}}}
	_, user := ai.BuildNarrationPrompt(clip.NarrationInput{PlanningInput: in, Flow: flow}, clip.DefaultCompositionLimits())
	var payload struct {
		Declared []struct {
			Element string `json:"element_id"`
			Chars   int    `json:"chars"`
		} `json:"declared_captions"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Declared) != 2 || payload.Declared[0].Element != "line" || payload.Declared[0].Chars != 7 || payload.Declared[1].Element != "free" || payload.Declared[1].Chars != 0 {
		t.Fatal(payload.Declared)
	}
}
