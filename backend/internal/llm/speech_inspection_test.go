package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

type nativeWitnessProvider struct {
	calls int
	err   error
}

func (p *nativeWitnessProvider) DesignVoice(context.Context, llm.VoiceDesignRequest) (llm.VoiceDesignResponse, error) {
	p.calls++
	return llm.VoiceDesignResponse{Evidence: llm.SpeechEvidence{RequestID: "private-supplier-request", ReportedUSD: "999"}}, p.err
}
func (p *nativeWitnessProvider) ConfirmVoice(context.Context, llm.VoiceConfirmationRequest) (llm.VoiceConfirmationResponse, error) {
	p.calls++
	return llm.VoiceConfirmationResponse{Voice: "private-confirmed-handle"}, p.err
}
func (p *nativeWitnessProvider) SynthesizeSpeech(context.Context, llm.SpeechRequest) (llm.SpeechResponse, error) {
	p.calls++
	return llm.SpeechResponse{Audio: llm.EncodedAudio{Bytes: []byte("private-audio-body")}}, p.err
}

func nativeWitnessRegistry(t *testing.T, provider *nativeWitnessProvider) *llm.Registry {
	t.Helper()
	options := opts
	options.SpeechAdapters = map[string]llm.SpeechAdapterFactory{"elevenlabs": func(llm.SpeechAdapterConfig) (llm.SpeechProvider, error) { return provider, nil }}
	registry, err := llm.Parse([]byte(goodYAML+speechYAML), env(map[string]string{"TEST_KEY": "private-text-key", "SPEECH_KEY": "private-speech-key"}), adaptersWith(&fakeProvider{}), twoModels(), options)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func nativeComposition(mode string, fields ...llm.RequestNativeField) *llm.RequestComposition {
	return &llm.RequestComposition{Stage: "speech", Mode: mode, PromptVersion: "native-contract-v1", SchemaVersion: "native-result-v1", Composer: "spoken." + mode, Parser: "llm.native-audio", Consumer: "spoken.private-result", SourceFiles: []string{"backend/internal/voice/spoken/app/generation.go"}, Activation: "explicit admitted native operation", NativeFields: fields, Output: llm.OutputContractInspection{Name: "native-" + mode, Version: "native-result-v1"}}
}

func TestNativeSpeechInspectionUsesActualFieldsWithoutChatRolesOrPrivateSupplierData(t *testing.T) {
	provider := &nativeWitnessProvider{}
	registry := nativeWitnessRegistry(t, provider)
	model := llm.ModelRef{ProviderID: "elevenlabs", ModelID: "explicit-native-model"}
	description, preview := strings.Repeat("한", 20), strings.Repeat("안", 100)
	field := func(id, text string) llm.RequestNativeField {
		return llm.RequestNativeField{ID: id, Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "native-input", Text: text, SourceRefs: []string{"saved-spoken-profile"}}
	}
	design := llm.VoiceDesignRequest{Model: model, Description: description, PreviewText: preview, Composition: nativeComposition("design", field("description", description), field("preview_text", preview))}
	prepared, err := registry.PrepareVoiceDesign(t.Context(), design)
	if err != nil || provider.calls != 0 || prepared.Status != llm.InspectionPrepared || len(prepared.Fragments) != 0 || prepared.IssuedAt != nil {
		t.Fatalf("native preview executed or invented chat roles: %+v %v", prepared, err)
	}
	if *prepared.Measures.Characters != int64(utf8.RuneCountInString(description+preview)) || *prepared.Measures.UTF8Bytes != int64(len(description+preview)) || prepared.Conditions.MaxCompletionTokens != nil || prepared.Conditions.ReasoningEffort != nil {
		t.Fatal("native fields acquired fictitious token/effort conditions")
	}
	result, err := registry.DesignVoice(t.Context(), design)
	if err != nil || provider.calls != 1 || result.Inspection == nil || result.Inspection.Status != llm.InspectionCaptured || result.Inspection.IssuedAt == nil {
		t.Fatalf("observable design witness missing: %+v %v", result.Inspection, err)
	}
	design.Composition.NativeFields[0].Text = "edited after execution"
	design.Composition.NativeFields[0].SourceRefs[0] = "edited reference"
	if result.Inspection.NativeFields[0].Text != description || result.Inspection.NativeFields[0].SourceRefs[0] != "saved-spoken-profile" {
		t.Fatal("captured native input shares mutable caller metadata")
	}
	confirm := llm.VoiceConfirmationRequest{DesignModel: model, Candidate: "private-candidate-handle", Name: "Saved voice", Description: description, Composition: nativeComposition("confirm", field("name", "Saved voice"), field("description", description))}
	confirmed, err := registry.ConfirmVoice(t.Context(), confirm)
	if err != nil || confirmed.Inspection == nil {
		t.Fatal("confirmation witness missing", err)
	}
	settings := llm.SpeechSettings{Stability: .5, SimilarityBoost: .75, Speed: 1}
	speech := llm.SpeechRequest{Model: model, Voice: "private-voice-handle", Text: "제공한 발화문", Settings: settings, Composition: nativeComposition("synthesize", field("text", "제공한 발화문"), field("settings", llm.SafeSpeechSettingsText(settings)))}
	if _, err := registry.PrepareSpeech(t.Context(), speech); err != nil || provider.calls != 2 {
		t.Fatal("native synthesis preview executed", err)
	}
	synthesized, err := registry.SynthesizeSpeech(t.Context(), speech)
	if err != nil || synthesized.Inspection == nil || provider.calls != 3 {
		t.Fatal("synthesis witness missing", err)
	}
	for _, inspection := range []*llm.RequestInspection{result.Inspection, confirmed.Inspection, synthesized.Inspection} {
		if err := inspection.Validate(); err != nil || len(inspection.Fragments) != 0 || inspection.Measures.ProviderPromptTokens != nil || inspection.Measures.ProviderCompletionTokens != nil {
			t.Fatal("native evidence became token/chat evidence", err)
		}
		raw, _ := json.Marshal(inspection)
		for _, private := range []string{"private-candidate-handle", "private-voice-handle", "private-confirmed-handle", "private-audio-body", "private-supplier-request", "private-text-key", "private-speech-key", "api.elevenlabs.io", "999"} {
			if strings.Contains(string(raw), private) {
				t.Fatal("native projection leaked supplier/media data", private)
			}
		}
	}
}

func TestNativeSpeechWitnessDistinguishesPreflightCancellationIssuedFailureAndMissingManifest(t *testing.T) {
	provider := &nativeWitnessProvider{}
	registry := nativeWitnessRegistry(t, provider)
	model := llm.ModelRef{ProviderID: "elevenlabs", ModelID: "explicit-native-model"}
	text := "허용된 발화문"
	request := llm.SpeechRequest{Model: model, Voice: "private-handle", Text: text, Settings: llm.SpeechSettings{Speed: 1}, Composition: nativeComposition("synthesize", llm.RequestNativeField{ID: "text", Authorship: llm.FragmentAuthorshipAccount, MaterialRole: "narration", Text: text})}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := registry.SynthesizeSpeech(cancelled, request); !errors.Is(err, context.Canceled) || result.Inspection != nil || provider.calls != 0 {
		t.Fatal("preflight cancellation claimed issued speech", err)
	}
	invalid := request
	invalid.Settings.Speed = 2
	if result, err := registry.SynthesizeSpeech(t.Context(), invalid); !errors.Is(err, llm.ErrUnsupported) || result.Inspection != nil || provider.calls != 0 {
		t.Fatal("invalid native request claimed issued speech", err)
	}
	failure := errors.New("native adapter failed after invocation")
	provider.err = failure
	result, err := registry.SynthesizeSpeech(t.Context(), request)
	witness, found := llm.RequestInspectionFromError(err)
	if !errors.Is(err, failure) || !found || witness.Status != llm.InspectionCaptured || result.Inspection == nil || provider.calls != 1 || strings.Contains(err.Error(), text) {
		t.Fatal("issued failure lost its original cause or safe witness", err)
	}
	provider.err = nil
	request.Composition = nil
	result, err = registry.SynthesizeSpeech(t.Context(), request)
	if err != nil || result.Inspection != nil || provider.calls != 2 {
		t.Fatal("missing historical manifest was reconstructed", err)
	}
	request.Composition = &llm.RequestComposition{}
	if _, err := registry.PrepareSpeech(t.Context(), request); !errors.Is(err, llm.ErrInvalidInspection) {
		t.Fatal("broken composition was silently called a valid preview", err)
	}
	result, err = registry.SynthesizeSpeech(t.Context(), request)
	if err != nil || result.Inspection != nil || provider.calls != 3 {
		t.Fatal("optional malformed witness discarded usable native work", err)
	}
}
