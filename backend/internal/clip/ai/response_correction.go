package ai

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

const responseContract = `
Emit one complete JSON object: no markdown, prose, comments, extra keys or nulls. Use every required key with exact case/type. Empty arrays=[] and strings="". Use finite numbers and integer milliseconds, not seconds/strings. Copy supplied IDs exactly. Check the closed schema before answering.
`

// Correct only model-generated parse/schema/field failures. Deterministic input,
// timeline feasibility, provider/transport and media failures are not repair calls.
func responseCorrection(err error, response llm.Response) bool {
	if err == nil || !response.Usage.CostReported || response.FinishReason == "length" || response.FinishReason == "content_filter" || !errors.Is(err, llm.ErrBadOutput) {
		return false
	}
	d, ok := clip.DiagnosticFromError(err)
	if ok && (d.Phase == "timeline_grow" || d.Phase == "timeline_shrink" || d.Phase == "timeline_total" || d.Phase == "input") {
		return false
	}
	return true
}

func completeValidated[T any](ctx context.Context, s *Service, model llm.ModelRef, request llm.Request, user string, policy llm.CallPolicy, parse func(string) (T, error)) (T, llm.Usage, error) {
	var zero T
	var usage llm.Usage
	usage.CostReported = true
	base := request.System
	for attempt := 0; attempt <= policy.ResponseRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, usage, err
		}
		if err := validatePrompt(request.System, user, request.JSONSchema, request.Execution.Delivery, policy.InputTokenLimit()); err != nil {
			return zero, usage, err
		}
		response, err := s.models.Complete(ctx, model, request)
		usage.PromptTokens += response.Usage.PromptTokens
		usage.CompletionTokens += response.Usage.CompletionTokens
		usage.ReasoningTokens += response.Usage.ReasoningTokens
		usage.CostMicrousd += response.Usage.CostMicrousd
		usage.CostReported = usage.CostReported && response.Usage.CostReported
		if err != nil {
			return zero, usage, err
		}
		result, err := parse(response.Text)
		if err == nil {
			return result, usage, nil
		}
		classified := llm.ResponseParseError(response, err)
		d, ok := clip.DiagnosticFromError(err)
		if !ok {
			d = clip.AttemptDiagnostic{Check: "output_shape", Phase: "decode"}
		}
		d.Values = clip.SafeAttemptValues(d.Values)
		d.Values["retry"] = attempt
		d.Values["retry_limit"] = policy.ResponseRetries
		classified = clip.WithAttemptDiagnostic(classified, d)
		if attempt == policy.ResponseRetries || !responseCorrection(err, response) {
			return zero, usage, classified
		}
		if err := clip.ReportResponseCorrection(ctx, attempt+1, policy.ResponseRetries, d); err != nil {
			return zero, usage, err
		}
		// The error category and numeric facts are owned/allowlisted. Raw output is
		// deliberately excluded: it cannot become a new instruction or enlarge input.
		feedback := promptJSON(map[string]any{"check": clip.SafeAttemptCheck(d.Check), "phase": clip.SafeAttemptPhase(d.Phase), "measurements": clip.SafeAttemptValues(d.Values)})
		request.System = base + "\nThe previous candidate failed validation. Produce a fresh complete response correcting this validation feedback, while preserving the original contract and factual input:\n" + feedback
	}
	return zero, usage, llm.ErrBadOutput
}
