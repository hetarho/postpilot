package ai

import "github.com/postpilot/backend/internal/llm"

// outputError contains only a code-owned category, never response content.
type outputError string

func (e outputError) Error() string                { return "clip output: " + string(e) }
func (e outputError) Unwrap() error                { return llm.ErrBadOutput }
func (e outputError) OutputValidationCode() string { return string(e) }
