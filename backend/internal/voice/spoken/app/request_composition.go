package app

import (
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice/spoken"
)

func spokenDescriptor(mode string) llm.RequestComposition {
	c := llm.RequestComposition{Stage: "spoken-voice", Mode: mode, PromptVersion: "spoken-native-v1", SchemaVersion: "spoken-native-v1", Composer: "spoken/app." + mode + "Request", Parser: "llm speech product validation and private audio validation", Consumer: "owned spoken operation publication or qualification probe", Activation: "explicit admitted owned qualified frozen speech profile; no chat role contract", SourceFiles: []string{"internal/voice/spoken/app/generation.go", "internal/voice/spoken/app/probe.go"}, Output: llm.OutputContractInspection{Name: "spoken-" + mode, Version: "spoken-native-v1"}}
	switch mode {
	case "design":
		c.Composer = "spoken/app.designRequest"
		c.Output.Name = "voice-design-candidates"
	case "confirm":
		c.Composer = "spoken/app.confirmRequest"
		c.Output.Name = "confirmed-spoken-voice"
	case "speech":
		c.Composer = "spoken/app.speechRequest"
		c.Output.Name = "speech-audio-timing"
		c.Activation += "; two distinct frozen qualification texts, cached compatible audio makes no call"
	}
	return c
}
func RequestCompositions() []llm.RequestComposition {
	o := spoken.Operation{Name: "Synthetic sound", Description: "Synthetic gentle sound direction", PreviewText: "Synthetic preview sentence", Texts: [2]string{"Synthetic first spoken text", "Synthetic second spoken text"}, Profile: spoken.Profile{Settings: llm.SpeechSettings{Stability: .5, SimilarityBoost: .5, Style: .2, SpeakerBoost: true, Speed: 1}}}
	return []llm.RequestComposition{*designRequest(o).Composition, *confirmRequest(o).Composition, *speechRequest(o, 0).Composition}
}

func spokenComposition(mode string, o spoken.Operation, index int) *llm.RequestComposition {
	c := spokenDescriptor(mode)
	add := func(id, material, text string, author llm.FragmentAuthorship) {
		c.NativeFields = append(c.NativeFields, llm.RequestNativeField{ID: id, Authorship: author, MaterialRole: material, Text: text, SourceFiles: []string{"internal/voice/spoken/app/generation.go"}})
	}
	switch mode {
	case "design":
		add("description", "owner-voice-design-direction", o.Description, llm.FragmentAuthorshipAccount)
		add("preview_text", "spoken-preview-text-not-experience-evidence", o.PreviewText, llm.FragmentAuthorshipAccount)
	case "confirm":
		add("name", "owner-voice-library-name", o.Name, llm.FragmentAuthorshipAccount)
		add("description", "owner-voice-design-direction", o.Description, llm.FragmentAuthorshipAccount)
	case "speech":
		add("text", "frozen-spoken-text-not-experience-evidence", o.Texts[index], llm.FragmentAuthorshipAccount)
		add("settings", "frozen-product-profile-settings", llm.SafeSpeechSettingsText(o.Profile.Settings), llm.FragmentAuthorshipCode)
	}
	return &c
}
