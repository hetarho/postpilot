package generation

import (
	"encoding/json"
	"fmt"
)

type purposePayload struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Instructions string `json:"instructions"`
}

type generationPayload struct {
	TargetLanguage string          `json:"target_language"`
	TargetLength   *int            `json:"target_length,omitempty"`
	Purpose        *purposePayload `json:"purpose,omitempty"`
	// The applicable guideline texts in injection order, frozen exactly as Purpose is. A
	// payload written before guidelines existed decodes with this absent, which is "none".
	Guidelines []string `json:"guidelines,omitempty"`
}

// GenerationOptions is what a durable generate job froze at enqueue. Every field is an
// option of that one run: a later edit of the post's target length, of the purpose row, or
// of any guideline must not change the prompt of work already waiting in the queue.
type GenerationOptions struct {
	TargetLanguage Language
	TargetLength   *int
	Purpose        *PurposeBrief
	Guidelines     []string
}

// EncodeGenerationPayload freezes generation-only options in the durable job.
func EncodeGenerationPayload(options GenerationOptions) ([]byte, error) {
	if !options.TargetLanguage.Valid() {
		return nil, ErrLanguageRequired
	}
	return json.Marshal(generationPayload{
		TargetLanguage: options.TargetLanguage.String(),
		TargetLength:   cloneOptionalInt(options.TargetLength),
		Purpose:        encodePurpose(options.Purpose),
		Guidelines:     cloneTexts(options.Guidelines),
	})
}

// DecodeGenerationPayload accepts an empty payload for jobs queued before this
// contract existed; those jobs intentionally carry no target length and no purpose.
// A payload written before purposes existed simply decodes with the field absent.
func DecodeGenerationPayload(raw []byte) (GenerationOptions, error) {
	if len(raw) == 0 {
		return GenerationOptions{TargetLanguage: LanguageKorean}, nil
	}
	var payload generationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return GenerationOptions{}, fmt.Errorf("decode generation payload: %w", err)
	}
	if payload.TargetLength != nil && *payload.TargetLength <= 0 {
		return GenerationOptions{}, fmt.Errorf("decode generation payload: target length must be positive")
	}
	// Payloads queued before language support did not carry this field. Migration and
	// compatibility both preserve their established Korean behavior.
	language := LanguageKorean
	if payload.TargetLanguage != "" {
		var err error
		language, err = ParseLanguage(payload.TargetLanguage)
		if err != nil {
			return GenerationOptions{}, fmt.Errorf("decode generation payload: %w", err)
		}
	}
	return GenerationOptions{
		TargetLanguage: language,
		TargetLength:   cloneOptionalInt(payload.TargetLength),
		Purpose:        decodePurpose(payload.Purpose),
		Guidelines:     cloneTexts(payload.Guidelines),
	}, nil
}

// cloneTexts keeps a frozen slice frozen: the payload and the job must not share backing
// storage with whatever the caller does next to its own slice.
func cloneTexts(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func encodePurpose(brief *PurposeBrief) *purposePayload {
	if brief == nil {
		return nil
	}
	return &purposePayload{Name: brief.Name, Description: brief.Description, Instructions: brief.Instructions}
}

func decodePurpose(payload *purposePayload) *PurposeBrief {
	if payload == nil {
		return nil
	}
	return &PurposeBrief{Name: payload.Name, Description: payload.Description, Instructions: payload.Instructions}
}
