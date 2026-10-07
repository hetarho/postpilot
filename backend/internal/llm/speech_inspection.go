package llm

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// SafeSpeechSettingsText describes the product's explicit synthesis settings.
// No supplier handle, account identity, audio or endpoint is available to this
// allowlisted projection. Invalid settings remain a preflight refusal.
func SafeSpeechSettingsText(settings SpeechSettings) string {
	if settings.Validate() != nil {
		return ""
	}
	raw, _ := json.Marshal(struct {
		Stability       float64 `json:"stability"`
		SimilarityBoost float64 `json:"similarity_boost"`
		Style           float64 `json:"style"`
		SpeakerBoost    bool    `json:"speaker_boost"`
		Speed           float64 `json:"speed"`
	}{settings.Stability, settings.SimilarityBoost, settings.Style, settings.SpeakerBoost, settings.Speed})
	return string(raw)
}

// Speech preparation validates the same explicit native operation as execution.
// Its fields have no chat-message role, token budget or reasoning vocabulary.
func (r *Registry) PrepareVoiceDesign(ctx context.Context, request VoiceDesignRequest) (RequestInspection, error) {
	if err := ctx.Err(); err != nil {
		return RequestInspection{}, err
	}
	if err := r.resolveSpeech(request.Model.ProviderID); err != nil {
		return RequestInspection{}, err
	}
	if err := request.Validate(); err != nil {
		return RequestInspection{}, err
	}
	return prepareNativeSpeech(request.Composition, request.Model)
}

func (r *Registry) PrepareVoiceConfirmation(ctx context.Context, request VoiceConfirmationRequest) (RequestInspection, error) {
	if err := ctx.Err(); err != nil {
		return RequestInspection{}, err
	}
	if err := r.resolveSpeech(request.DesignModel.ProviderID); err != nil {
		return RequestInspection{}, err
	}
	if err := request.Validate(); err != nil {
		return RequestInspection{}, err
	}
	return prepareNativeSpeech(request.Composition, request.DesignModel)
}

func (r *Registry) PrepareSpeech(ctx context.Context, request SpeechRequest) (RequestInspection, error) {
	if err := ctx.Err(); err != nil {
		return RequestInspection{}, err
	}
	if err := r.resolveSpeech(request.Model.ProviderID); err != nil {
		return RequestInspection{}, err
	}
	if err := request.Validate(); err != nil {
		return RequestInspection{}, err
	}
	return prepareNativeSpeech(request.Composition, request.Model)
}

func prepareNativeSpeech(composition *RequestComposition, model ModelRef) (RequestInspection, error) {
	if composition == nil {
		return UnavailableRequestInspection("", ""), nil
	}
	prepared, err := PreparedCompositionInspection(composition)
	if err != nil {
		return RequestInspection{}, err
	}
	prepared.Conditions = &EffectiveRequestConditions{Model: &model}
	return prepared, prepared.Validate()
}

func capturedSpeechInspection(prepared RequestInspection, invokedAt time.Time) *RequestInspection {
	if prepared.Status == InspectionUnavailable {
		return nil
	}
	// Native operations report billing units through their separate evidence;
	// those units and supplier charges never become fictitious token counts.
	captured, err := CapturedRequestInspection(prepared, invokedAt, Usage{})
	if err != nil {
		return nil
	}
	return &captured
}

func inspectedSpeechError(err error, inspection *RequestInspection) error {
	if err == nil || inspection == nil {
		return err
	}
	return WithRequestInspectionError(err, *inspection)
}

func speechExecutionPreparation(prepared RequestInspection, err error) (RequestInspection, error) {
	// A broken optional inspection witness cannot discard a valid paid operation.
	// Public preparation still reports malformed metadata so the composer can fix
	// it; execution preserves the result with unavailable historical inspection.
	if errors.Is(err, ErrInvalidInspection) {
		return UnavailableRequestInspection("", ""), nil
	}
	return prepared, err
}
