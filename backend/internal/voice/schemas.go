package voice

import _ "embed"

//go:embed schemas/voice_analysis.schema.json
var voiceAnalysisSchema []byte

//go:embed schemas/voice_analysis_en.schema.json
var englishVoiceAnalysisSchema []byte

// VoiceAnalysisSchema is the response contract for the learn_voice completion, attached only
// when the resolved model declares structured output (mirrors generation/schemas.go).
func VoiceAnalysisSchema() []byte { return append([]byte(nil), voiceAnalysisSchema...) }

func VoiceAnalysisSchemaForLanguage(language Language) []byte {
	if language == LanguageEnglish {
		return append([]byte(nil), englishVoiceAnalysisSchema...)
	}
	return VoiceAnalysisSchema()
}
