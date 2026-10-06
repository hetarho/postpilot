package store

import (
	"encoding/json"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// Browser-only admission metadata shares the existing immutable speech column
// so no speculative schema column is needed. Other speech/result codecs remain
// array-only. Legacy browser rows still decode their original array shape.
type browserCompositionEnvelope struct {
	Version     int                             `json:"version"`
	Composition clip.BrowserCompositionContract `json:"composition"`
	Speech      []clip.SpeechPlacement          `json:"speech"`
}

func encodeBrowserComposition(r clip.BrowserRender) (string, error) {
	if r.Composition == nil {
		return encodeResultSpeech(r.Speech)
	}
	if err := r.Composition.Validate(true); err != nil {
		return "", err
	}
	if _, err := encodeResultSpeech(r.Speech); err != nil {
		return "", err
	}
	data, err := json.Marshal(browserCompositionEnvelope{Version: 1, Composition: *r.Composition, Speech: r.Speech})
	if err != nil || len(data) > clip.SpeechManifestMaxBytes {
		return "", clip.ErrInvalidMedia
	}
	return string(data), nil
}

func decodeBrowserComposition(raw string) ([]clip.SpeechPlacement, *clip.BrowserCompositionContract, error) {
	if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		speech, err := decodeResultSpeech(raw)
		return speech, nil, err
	}
	var envelope browserCompositionEnvelope
	if len(raw) > clip.SpeechManifestMaxBytes || clip.StrictJSON(raw, &envelope) != nil || envelope.Version != 1 {
		return nil, nil, clip.ErrInvalidMedia
	}
	if err := envelope.Composition.Validate(true); err != nil {
		return nil, nil, err
	}
	if _, err := encodeResultSpeech(envelope.Speech); err != nil {
		return nil, nil, err
	}
	return envelope.Speech, &envelope.Composition, nil
}
