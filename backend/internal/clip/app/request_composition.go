package app

import "github.com/postpilot/backend/internal/llm"

func clipSpeechDescriptor() llm.RequestComposition {
	return llm.RequestComposition{Stage: "video-speech", Mode: "segment-synthesis", PromptVersion: "clip-speech-native-v1", SchemaVersion: "clip-speech-native-v1", Composer: "clip/app.segmentSpeechRequest", Parser: "speech evidence/timing validation and audio validation", Consumer: "private segment speech asset aligned into edit plan", Activation: "explicit admitted missing or changed segment; compatible stored audio stays playable without a new call", SourceFiles: []string{"internal/clip/app/speech.go"}, Output: llm.OutputContractInspection{Name: "speech-audio-timing", Version: "clip-speech-native-v1"}}
}
func RequestCompositions() []llm.RequestComposition {
	return []llm.RequestComposition{*segmentSpeechRequest(llm.ModelRef{}, "", "Synthetic reviewed spoken segment", llm.SpeechSettings{Stability: .5, SimilarityBoost: .5, Speed: 1}, "synthetic-segment").Composition}
}
func segmentSpeechRequest(model llm.ModelRef, handle llm.VoiceHandle, text string, settings llm.SpeechSettings, segment string) llm.SpeechRequest {
	c := clipSpeechDescriptor()
	refs := []string(nil)
	if segment != "" {
		refs = []string{segment}
	}
	c.NativeFields = []llm.RequestNativeField{{ID: "text", Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "frozen-reviewed-spoken-segment", Text: text, SourceRefs: refs, SourceFiles: c.SourceFiles}, {ID: "settings", Authorship: llm.FragmentAuthorshipCode, MaterialRole: "frozen-product-profile-settings", Text: llm.SafeSpeechSettingsText(settings), SourceFiles: c.SourceFiles}}
	return llm.SpeechRequest{Composition: &c, Model: model, Voice: handle, Text: text, Settings: settings}
}
