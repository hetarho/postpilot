package rpc

import (
	"errors"
	"strconv"

	"connectrpc.com/connect"

	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

// Target domains still own validation. The test edge carries the same useful,
// bounded refusal reasons without private source text or provider diagnostics.
func writingTargetError(err error) (error, bool) {
	var cap *guideline.AccountCapError
	var title *guideline.TitleTooLongError
	var text *guideline.TextTooLongError
	var field *template.FieldTooLongError
	var number *template.NumberOutOfRangeError
	var name *voice.VoiceNameError
	makeError := func(code connect.Code, message string, reason v1.FailureReason, params map[string]string) (error, bool) {
		return rpcserver.NewAppError(code, message, reason, params), true
	}
	switch {
	case errors.As(err, &cap):
		return makeError(connect.CodeFailedPrecondition, "guideline limit reached", v1.FailureReason_GUIDELINE_LIMIT_REACHED, map[string]string{"max": strconv.Itoa(cap.Max)})
	case errors.As(err, &title):
		return makeError(connect.CodeInvalidArgument, "guideline title is too long", v1.FailureReason_GUIDELINE_TITLE_TOO_LONG, map[string]string{"max": strconv.Itoa(title.Max), "actual": strconv.Itoa(title.Chars)})
	case errors.As(err, &text):
		return makeError(connect.CodeInvalidArgument, "guideline text is too long", v1.FailureReason_GUIDELINE_TEXT_TOO_LONG, map[string]string{"max": strconv.Itoa(text.Max), "actual": strconv.Itoa(text.Chars)})
	case errors.As(err, &field):
		return makeError(connect.CodeInvalidArgument, "template field is too long", v1.FailureReason_TEMPLATE_FIELD_TOO_LONG, map[string]string{"field": field.Field, "max": strconv.Itoa(field.Max), "actual": strconv.Itoa(field.Chars)})
	case errors.As(err, &number):
		return makeError(connect.CodeInvalidArgument, "template number is out of range", v1.FailureReason_TEMPLATE_NUMBER_OUT_OF_RANGE, map[string]string{"field": number.Field, "min": strconv.Itoa(number.Min), "max": strconv.Itoa(number.Max), "actual": strconv.Itoa(number.Value)})
	case errors.As(err, &name):
		if name.Chars == 0 {
			return makeError(connect.CodeInvalidArgument, "voice name is required", v1.FailureReason_VOICE_NAME_REQUIRED, nil)
		}
		return makeError(connect.CodeInvalidArgument, "voice name is too long", v1.FailureReason_VOICE_NAME_TOO_LONG, map[string]string{"actual": strconv.Itoa(name.Chars), "max": strconv.Itoa(voice.VoiceNameMaxChars)})
	case errors.Is(err, template.ErrTooMany):
		return makeError(connect.CodeFailedPrecondition, "template limit reached", v1.FailureReason_TEMPLATE_LIMIT_REACHED, nil)
	case errors.Is(err, template.ErrDuplicateName):
		return makeError(connect.CodeAlreadyExists, "template name already exists", v1.FailureReason_TEMPLATE_NAME_TAKEN, nil)
	case errors.Is(err, template.ErrNameRequired):
		return makeError(connect.CodeInvalidArgument, "template name is required", v1.FailureReason_TEMPLATE_NAME_REQUIRED, nil)
	case errors.Is(err, template.ErrNotFound):
		return makeError(connect.CodeNotFound, "template not found", v1.FailureReason_TEMPLATE_NOT_FOUND, nil)
	case errors.Is(err, guideline.ErrScopeShape):
		return makeError(connect.CodeInvalidArgument, "guideline scope is invalid", v1.FailureReason_GUIDELINE_SCOPE_INVALID, nil)
	case errors.Is(err, guideline.ErrDuplicateText):
		return makeError(connect.CodeAlreadyExists, "guideline text already exists", v1.FailureReason_GUIDELINE_TEXT_TAKEN, nil)
	case errors.Is(err, guideline.ErrTemplateNotFound):
		return makeError(connect.CodeNotFound, "scoped template not found", v1.FailureReason_GUIDELINE_TEMPLATE_NOT_FOUND, nil)
	case errors.Is(err, guideline.ErrFieldNotFound):
		return makeError(connect.CodeNotFound, "scoped field not found", v1.FailureReason_GUIDELINE_FIELD_NOT_FOUND, nil)
	case errors.Is(err, guideline.ErrInvalidText):
		return makeError(connect.CodeInvalidArgument, "guideline text is required", v1.FailureReason_GUIDELINE_TEXT_REQUIRED, nil)
	case errors.Is(err, guideline.ErrNotFound):
		return makeError(connect.CodeNotFound, "guideline not found", v1.FailureReason_GUIDELINE_NOT_FOUND, nil)
	case errors.Is(err, voice.ErrVoiceNameTaken):
		return makeError(connect.CodeAlreadyExists, "voice name already exists", v1.FailureReason_VOICE_NAME_TAKEN, nil)
	case errors.Is(err, voice.ErrVoiceNotFound):
		return makeError(connect.CodeNotFound, "voice not found", v1.FailureReason_VOICE_NOT_FOUND, nil)
	case errors.Is(err, provider.ErrModelNotRegistered):
		return makeError(connect.CodeNotFound, "model not registered", v1.FailureReason_MODEL_NOT_REGISTERED, nil)
	case errors.Is(err, provider.ErrModelDisabled):
		return makeError(connect.CodeFailedPrecondition, "model disabled", v1.FailureReason_MODEL_DISABLED, nil)
	case errors.Is(err, provider.ErrModelUnsuitable):
		return makeError(connect.CodeFailedPrecondition, "model unsuitable", v1.FailureReason_MODEL_UNSUITABLE, nil)
	case errors.Is(err, post.ErrNotFound), errors.Is(err, post.ErrForbidden):
		return makeError(connect.CodeNotFound, "source post not found", v1.FailureReason_POST_NOT_FOUND, nil)
	case errors.Is(err, post.ErrPostPublished):
		return makeError(connect.CodeFailedPrecondition, "source post is published", v1.FailureReason_POST_PUBLISHED_LOCKED, nil)
	case errors.Is(err, post.ErrPostBusy):
		return makeError(connect.CodeFailedPrecondition, "source post has an active job", v1.FailureReason_POST_BUSY, nil)
	}
	return nil, false
}
