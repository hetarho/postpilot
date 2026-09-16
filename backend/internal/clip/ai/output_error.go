package ai

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// outputError contains only a code-owned category, never response content.
type outputError string

func (e outputError) Error() string                { return "clip output: " + string(e) }
func (e outputError) Unwrap() error                { return llm.ErrBadOutput }
func (e outputError) OutputValidationCode() string { return string(e) }

// footageError is a readable, schema-valid plan whose assembled output falls
// under the length floor (CLIP-120). It carries a validation code like any
// other check so attempt inspection can name it, but it unwraps to
// ErrInsufficientFootage rather than ErrBadOutput: the response was read and
// validated, so no correction attempt may be issued for it and the owner-facing
// cause points at the footage instead of the model.
type footageError string

func (e footageError) Error() string                { return "clip output: " + string(e) }
func (e footageError) Unwrap() error                { return clip.ErrInsufficientFootage }
func (e footageError) OutputValidationCode() string { return string(e) }
