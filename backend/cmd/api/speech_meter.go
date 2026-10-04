package main

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

func (m meteredRegistry) DesignVoice(ctx context.Context, req llm.VoiceDesignRequest) (llm.VoiceDesignResponse, error) {
	return (usage.SpeechMeter{Provider: m.Registry, Ledger: m.ledger}).DesignVoice(ctx, req)
}
func (m meteredRegistry) ConfirmVoice(ctx context.Context, req llm.VoiceConfirmationRequest) (llm.VoiceConfirmationResponse, error) {
	return (usage.SpeechMeter{Provider: m.Registry, Ledger: m.ledger}).ConfirmVoice(ctx, req)
}
func (m meteredRegistry) SynthesizeSpeech(ctx context.Context, req llm.SpeechRequest) (llm.SpeechResponse, error) {
	return (usage.SpeechMeter{Provider: m.Registry, Ledger: m.ledger}).SynthesizeSpeech(ctx, req)
}
