package main

import (
	"context"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestCompositionSpeechPortsCannotReachAnUnmeteredProvider(t *testing.T) {
	m := meteredRegistry{}
	ref := llm.ModelRef{ProviderID: "p", ModelID: "d"}
	if _, err := m.DesignVoice(context.Background(), llm.VoiceDesignRequest{Model: ref, Description: strings.Repeat("a", 20), PreviewText: strings.Repeat("a", 100)}); err == nil {
		t.Fatal("unmetered design")
	}
	if _, err := m.ConfirmVoice(context.Background(), llm.VoiceConfirmationRequest{DesignModel: ref, Candidate: "candidate", Name: "name", Description: strings.Repeat("a", 20)}); err == nil {
		t.Fatal("unmetered confirmation")
	}
	if _, err := m.SynthesizeSpeech(context.Background(), llm.SpeechRequest{Model: ref, Voice: "voice", Text: "text", Settings: llm.SpeechSettings{Speed: 1}}); err == nil {
		t.Fatal("unmetered synthesis")
	}
}
