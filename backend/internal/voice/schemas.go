package voice

import _ "embed"

//go:embed schemas/voice_analysis.schema.json
var voiceAnalysisSchema []byte

// VoiceAnalysisSchema is the response contract for the learn_voice completion, attached only
// when the resolved model declares structured output (mirrors generation/schemas.go).
func VoiceAnalysisSchema() []byte { return append([]byte(nil), voiceAnalysisSchema...) }
