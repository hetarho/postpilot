package llm

import (
	"errors"
	"strings"
	"time"
)

// PreparedCompositionInspection never reads transport bodies. Safe text and
// ordered identifiers must be declared by the owning composer explicitly.
func PreparedCompositionInspection(c *RequestComposition) (RequestInspection, error) {
	if c == nil {
		return UnavailableRequestInspection("", ""), nil
	}
	inspection := RequestInspection{Version: RequestInspectionVersion, Status: InspectionPrepared, Stage: c.Stage, Mode: c.Mode, PromptVersion: c.PromptVersion, SchemaVersion: c.SchemaVersion, Composer: c.Composer, Parser: c.Parser, Consumer: c.Consumer, Activation: c.Activation, SourceFiles: c.SourceFiles, Fragments: c.Fragments, NativeFields: c.NativeFields, SelectedRuleIDs: c.SelectedRuleIDs, Omissions: c.Omissions, Output: c.Output}
	inspection = cloneRequestInspection(inspection)
	var measured strings.Builder
	for _, fragment := range inspection.Fragments {
		measured.WriteString(fragment.Text)
	}
	for _, field := range inspection.NativeFields {
		measured.WriteString(field.Text)
	}
	measure, err := MeasureInspectionText(measured.String())
	if err != nil {
		return RequestInspection{}, err
	}
	inspection.Measures = measure
	if c.ReferenceTokenEstimate != nil {
		value := *c.ReferenceTokenEstimate
		inspection.Measures.ReferenceTokenEstimate = &value
	}
	if err := inspection.Validate(); err != nil {
		return RequestInspection{}, err
	}
	return inspection, nil
}
func PreparedRequestInspection(req Request) (RequestInspection, error) {
	inspection, err := PreparedCompositionInspection(req.Composition)
	if err != nil || inspection.Status == InspectionUnavailable {
		return inspection, err
	}
	// The product schema at execution is authoritative; omitted structured output
	// remains the composer's named plain-text contract.
	if req.JSONSchema != nil {
		inspection.Output.Schema = string(req.JSONSchema)
	}
	// Count the exact application text, including composer envelopes. Semantic
	// fragments may group or omit those wrappers; they are the safe projection,
	// not a tokenizer input. No media body, URL, schema or SDK framing is counted.
	var text strings.Builder
	text.WriteString(req.System)
	for _, message := range req.Messages {
		for _, part := range message.Parts {
			if !part.IsImage() && !part.IsVideo() {
				text.WriteString(part.Text)
			}
		}
	}
	measure, err := MeasureInspectionText(text.String())
	if err != nil {
		return RequestInspection{}, err
	}
	inspection.Measures.Characters = measure.Characters
	inspection.Measures.UTF8Bytes = measure.UTF8Bytes
	return inspection, inspection.Validate()
}

// CapturedRequestInspection records an observable adapter invocation, not
// network acceptance. Existing Usage zeros mean unreported, so they stay unknown.
func CapturedRequestInspection(prepared RequestInspection, invokedAt time.Time, reported Usage) (RequestInspection, error) {
	if prepared.Status == InspectionUnavailable {
		return prepared, nil
	}
	result := cloneRequestInspection(prepared)
	result.Status = InspectionCaptured
	result.IssuedAt = &invokedAt
	if reported.PromptTokens > 0 {
		count := int64(reported.PromptTokens)
		result.Measures.ProviderPromptTokens = &count
	}
	if reported.CompletionTokens > 0 {
		count := int64(reported.CompletionTokens)
		result.Measures.ProviderCompletionTokens = &count
	}
	if reported.ReasoningTokens > 0 {
		count := int64(reported.ReasoningTokens)
		result.Measures.ProviderReasoningTokens = &count
	}
	return result, result.Validate()
}

type requestInspectionError struct {
	cause      error
	inspection RequestInspection
}

func (e *requestInspectionError) Error() string { return e.cause.Error() }
func (e *requestInspectionError) Unwrap() error { return e.cause }
func WithRequestInspectionError(err error, inspection RequestInspection) error {
	if err == nil {
		return nil
	}
	if inspection.Status != InspectionCaptured {
		return err
	}
	return &requestInspectionError{cause: err, inspection: cloneRequestInspection(inspection)}
}
func RequestInspectionFromError(err error) (RequestInspection, bool) {
	if err == nil {
		return RequestInspection{}, false
	}
	// An adapter's error chain is available to errors.Is/As, while its claimed
	// witness is fenced. Only a registry wrapper outside that boundary may win.
	if _, blocked := err.(*providerInspectionBoundary); blocked {
		return RequestInspection{}, false
	}
	if evidence, ok := err.(*requestInspectionError); ok {
		return cloneRequestInspection(evidence.inspection), true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return RequestInspectionFromError(wrapped.Unwrap())
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if inspection, found := RequestInspectionFromError(child); found {
				return inspection, true
			}
		}
	}
	return RequestInspection{}, false
}

type providerInspectionBoundary struct{ cause error }

func (e *providerInspectionBoundary) Error() string { return e.cause.Error() }
func (e *providerInspectionBoundary) Unwrap() error { return e.cause }

// Fence only errors containing a known witness wrapper. Preserve the complete
// original error identity and message without exposing adapter-forged evidence.
func stripProviderInspectionError(err error) error {
	var witness *requestInspectionError
	if err == nil || !errors.As(err, &witness) {
		return err
	}
	return &providerInspectionBoundary{cause: err}
}

// CloneRequestInspection gives private payload owners an independent safe
// projection. It grants no authority to turn prepared data into issued history.
func CloneRequestInspection(in RequestInspection) RequestInspection {
	return cloneRequestInspection(in)
}

func cloneRequestInspection(in RequestInspection) RequestInspection {
	copy := in
	copy.Attachments = append([]InspectionAttachment(nil), in.Attachments...)
	copy.SourceFiles = append([]string(nil), in.SourceFiles...)
	copy.SelectedRuleIDs = append([]string(nil), in.SelectedRuleIDs...)
	copy.Fragments = make([]RequestFragment, len(in.Fragments))
	for i, f := range in.Fragments {
		copy.Fragments[i] = f
		copy.Fragments[i].SourceRefs = append([]string(nil), f.SourceRefs...)
		copy.Fragments[i].SourceFiles = append([]string(nil), f.SourceFiles...)
	}
	copy.NativeFields = make([]RequestNativeField, len(in.NativeFields))
	for i, f := range in.NativeFields {
		copy.NativeFields[i] = f
		copy.NativeFields[i].SourceRefs = append([]string(nil), f.SourceRefs...)
		copy.NativeFields[i].SourceFiles = append([]string(nil), f.SourceFiles...)
	}
	copy.Omissions = make([]RequestOmission, len(in.Omissions))
	for i, o := range in.Omissions {
		copy.Omissions[i] = o
		copy.Omissions[i].SourceFiles = append([]string(nil), o.SourceFiles...)
	}
	if in.IssuedAt != nil {
		value := *in.IssuedAt
		copy.IssuedAt = &value
	}
	if in.Conditions != nil {
		c := *in.Conditions
		copy.Conditions = &c
		if c.Model != nil {
			v := *c.Model
			copy.Conditions.Model = &v
		}
		if c.MaxCompletionTokens != nil {
			v := *c.MaxCompletionTokens
			copy.Conditions.MaxCompletionTokens = &v
		}
		if c.ReasoningEffort != nil {
			v := *c.ReasoningEffort
			copy.Conditions.ReasoningEffort = &v
		}
		copy.Conditions.StructuredOutput = copyBool(c.StructuredOutput)
		copy.Conditions.DisableReasoning = copyBool(c.DisableReasoning)
		copy.Conditions.FreeCall = copyBool(c.FreeCall)
		copy.Conditions.DefaultBudget = copyBool(c.DefaultBudget)
		copy.Conditions.FrozenExecution = copyBool(c.FrozenExecution)
		copy.Conditions.ReasoningOmitted = copyBool(c.ReasoningOmitted)
	}
	copy.Measures = InspectionMeasures{Characters: copyCount(in.Measures.Characters), UTF8Bytes: copyCount(in.Measures.UTF8Bytes), ReferenceTokenEstimate: copyCount(in.Measures.ReferenceTokenEstimate), ProviderPromptTokens: copyCount(in.Measures.ProviderPromptTokens), ProviderCompletionTokens: copyCount(in.Measures.ProviderCompletionTokens), ProviderReasoningTokens: copyCount(in.Measures.ProviderReasoningTokens)}
	return copy
}
func copyBool(in *bool) *bool {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
func copyCount(in *int64) *int64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
