package clip

import (
	"bytes"
	"encoding/json"
	"io"
)

func ValidLanguage(language string) bool { return language == "ko" || language == "en" }

func SupportedGenerationPayload(version int) bool {
	return version >= 3 && version <= GenerationPayloadVersion
}

func (p *GenerationPayload) UnmarshalJSON(raw []byte) error {
	type wire GenerationPayload
	var value wire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	if (value.Version == 3 || value.Version == 4) && value.Language == "" {
		value.Language = "ko"
	}
	if !ValidLanguage(value.Language) {
		return ErrInvalid
	}
	*p = GenerationPayload(value)
	return nil
}
