package generation

import (
	"encoding/json"
	"fmt"
)

type generationPayload struct {
	TargetLength *int `json:"target_length,omitempty"`
}

// EncodeGenerationPayload freezes generation-only options in the durable job.
func EncodeGenerationPayload(targetLength *int) ([]byte, error) {
	return json.Marshal(generationPayload{TargetLength: cloneOptionalInt(targetLength)})
}

// DecodeGenerationPayload accepts an empty payload for jobs queued before this
// contract existed; those jobs intentionally carry no target length.
func DecodeGenerationPayload(raw []byte) (*int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var payload generationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode generation payload: %w", err)
	}
	if payload.TargetLength != nil && *payload.TargetLength <= 0 {
		return nil, fmt.Errorf("decode generation payload: target length must be positive")
	}
	return cloneOptionalInt(payload.TargetLength), nil
}
