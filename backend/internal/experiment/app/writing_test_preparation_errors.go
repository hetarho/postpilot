package app

import (
	"errors"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/usage"
)

// WritingTestPreparationError translates owner-domain refusals at the app seam.
// Provider/entitlement failures retain their existing safe typed vocabulary.
func WritingTestPreparationError(err error) error {
	if err == nil {
		return nil
	}
	var required *generation.RequiredTemplateAnswerError
	if errors.As(err, &required) {
		return &experiment.RequiredTemplateAnswerError{Label: required.Label}
	}
	var unsupported *generation.VideoUnsupportedError
	if errors.As(err, &unsupported) {
		return &experiment.VideoUnsupportedError{Model: unsupported.Model}
	}
	var length *post.TargetLengthError
	var answer *post.TemplateAnswerTooLongError
	switch {
	case errors.Is(err, generation.ErrWritingTestCount):
		return experiment.ErrTestCount
	case errors.Is(err, generation.ErrWritingTestFactor):
		return experiment.ErrTestFactor
	case errors.Is(err, generation.ErrWritingTestDuplicate):
		return experiment.ErrTestDuplicate
	case errors.Is(err, generation.ErrWritingTestRevision):
		return experiment.ErrTestRevisionConflict
	case errors.Is(err, generation.ErrWritingTestReference), errors.Is(err, usage.ErrPricingUnavailable):
		return experiment.ErrTestEntrant
	case errors.Is(err, generation.ErrWritingTestMaterial), errors.As(err, &length), errors.As(err, &answer), errors.Is(err, post.ErrInvalidTagCount), errors.Is(err, post.ErrTemplateAnswerInvalid), errors.Is(err, post.ErrQualityRuleInvalid):
		return experiment.ErrTestMaterialInvalid
	case errors.Is(err, generation.ErrWritingTestCheckpointInvalid), errors.Is(err, generation.ErrWritingTestExecutionUncertain), errors.Is(err, generation.ErrWritingTestRetryRequired):
		return experiment.ErrTestStateInvalid
	case errors.Is(err, generation.ErrLanguageRequired), errors.Is(err, post.ErrLanguageRequired):
		return experiment.ErrLanguageRequired
	case errors.Is(err, generation.ErrVideoUnsupported):
		return experiment.ErrVideoUnsupported
	case errors.Is(err, generation.ErrVoiceDeleted):
		return experiment.ErrVoiceUnavailable
	case errors.Is(err, generation.ErrVoiceNotMade):
		return experiment.ErrVoiceNotMade
	case errors.Is(err, generation.ErrVoiceMismatch):
		return experiment.ErrTestRevisionConflict
	case errors.Is(err, generation.ErrNotFound), errors.Is(err, generation.ErrForbidden):
		return experiment.ErrTestNotFound
	default:
		return err
	}
}
