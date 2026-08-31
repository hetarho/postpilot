package voice

import (
	"errors"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const (
	FailureReasonContentLanguageMismatch = "VOICE_CONTENT_LANGUAGE_MISMATCH"
	FailureReasonVoiceDeleted            = "VOICE_DELETED"
	FailureReasonVoiceNotFound           = "VOICE_NOT_FOUND"
	FailureReasonInvalidLifecycle        = "VOICE_INVALID_LIFECYCLE"
	FailureReasonUnknown                 = "UNKNOWN_FAILURE"
)

// Failure is the voice context's durable, localizable failure projection. Params is
// display-safe metadata only; provider diagnostics belong in TechnicalDetail.
type Failure struct {
	Reason          string
	Params          map[string]string
	TechnicalDetail string
}

func (f Failure) Empty() bool {
	return strings.TrimSpace(f.Reason) == "" && len(f.Params) == 0 && strings.TrimSpace(f.TechnicalDetail) == ""
}

func cloneFailure(value *Failure) *Failure {
	if value == nil {
		return nil
	}
	copy := *value
	if len(value.Params) > 0 {
		copy.Params = make(map[string]string, len(value.Params))
		for key, item := range value.Params {
			copy.Params[key] = item
		}
	}
	return &copy
}

func normalizeFailure(err error) Failure {
	var mismatch *ContentLanguageMismatchError
	switch {
	case errors.As(err, &mismatch):
		params := map[string]string{}
		if mismatch.ContentLanguage.Valid() {
			params["content_language"] = string(mismatch.ContentLanguage)
		}
		if mismatch.SourceLanguage.Valid() {
			params["source_language"] = string(mismatch.SourceLanguage)
		}
		return Failure{Reason: FailureReasonContentLanguageMismatch, Params: params}
	case errors.Is(err, ErrVoiceDeleted):
		return Failure{Reason: FailureReasonVoiceDeleted}
	case errors.Is(err, ErrVoiceNotFound), errors.Is(err, ErrVoiceRequired):
		return Failure{Reason: FailureReasonVoiceNotFound}
	case errors.Is(err, ErrInvalidLifecycle):
		return Failure{Reason: FailureReasonInvalidLifecycle}
	}
	normalized := llm.NormalizeFailure(err)
	return Failure{Reason: normalized.Reason, Params: normalized.Params, TechnicalDetail: normalized.TechnicalDetail}
}
