package app

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

func TestClipSegmentNativeCompositionUsesActualFrozenTextSettingsAndSafeID(t *testing.T) {
	settings := llm.SpeechSettings{Stability: .5, SimilarityBoost: .5, Speed: 1}
	req := segmentSpeechRequest(llm.ModelRef{ProviderID: "p", ModelID: "m"}, llm.VoiceHandle("secret-voice-handle"), "reviewed text", settings, "segment-1")
	got, err := llm.PreparedCompositionInspection(req.Composition)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if len(got.Fragments) != 0 || len(got.NativeFields) != 2 || got.NativeFields[0].Text != req.Text || got.NativeFields[0].SourceRefs[0] != "segment-1" || got.NativeFields[1].Text != llm.SafeSpeechSettingsText(req.Settings) || strings.Contains(string(raw), "secret-voice-handle") || len(RequestCompositions()) != 1 {
		t.Fatalf("native inventory changed request or exposed provider fields: %+v", got)
	}
}
