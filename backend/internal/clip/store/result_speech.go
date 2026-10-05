package store

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

func encodeResultSpeech(speech []clip.SpeechPlacement) (string, error) {
	if len(speech) == 0 {
		return "", nil
	}
	if len(speech) > clip.MaxSpokenSegments {
		return "", clip.ErrInvalidMedia
	}
	bytes, err := json.Marshal(speech)
	if err != nil || len(bytes) > clip.SpeechManifestMaxBytes {
		return "", clip.ErrInvalidMedia
	}
	return string(bytes), nil
}
func decodeResultSpeech(raw string) ([]clip.SpeechPlacement, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > clip.SpeechManifestMaxBytes {
		return nil, clip.ErrInvalidMedia
	}
	var speech []clip.SpeechPlacement
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&speech) != nil || len(speech) > clip.MaxSpokenSegments || decoder.Decode(new(any)) != io.EOF {
		return nil, clip.ErrInvalidMedia
	}
	return speech, nil
}
