package job

import (
	"errors"

	"github.com/postpilot/backend/internal/llm"
)

// testReporting is the same shape the composition root wires (cmd/api jobReporting): the
// provider's failure vocabulary, its diagnostics, and the one kind family whose errors
// are redacted. It is duplicated here so the queue's own tests can exercise the port
// without the queue depending on a provider.
type testReporting struct{}

func (testReporting) Failure(err error) Failure {
	normalized := llm.NormalizeFailure(err)
	var detailed interface{ Failure() llm.Failure }
	if errors.As(err, &detailed) {
		normalized = detailed.Failure()
	}
	return Failure{Reason: normalized.Reason, Params: normalized.Params, TechnicalDetail: normalized.TechnicalDetail}
}

func (testReporting) LogAttrs(err error) []any {
	diagnostic, ok := llm.DiagnosticOf(err)
	if !ok {
		return nil
	}
	attrs := []any{"operation", diagnostic.Operation, "error_class", diagnostic.Class}
	if diagnostic.HTTPStatus != 0 {
		attrs = append(attrs, "http_status", diagnostic.HTTPStatus)
	}
	if diagnostic.UpstreamCode != 0 {
		attrs = append(attrs, "upstream_code", diagnostic.UpstreamCode)
	}
	if diagnostic.RequestID != "" {
		attrs = append(attrs, "request_id", diagnostic.RequestID)
	}
	return attrs
}

func (testReporting) SafeStage(kind, stage string) (string, bool) {
	if !redactedTestKind(kind) {
		return "", false
	}
	switch stage {
	case "queued", "prepare", "analyze", "analyze_retry", "flow", "flow_retry", "narrate", "narrate_retry", "plan", "plan_retry", "render", "save", "cleanup":
		return stage, true
	}
	return "unknown", true
}

func (testReporting) Redacted(kind string) bool { return redactedTestKind(kind) }

func redactedTestKind(kind string) bool {
	return kind == "generate_clip" || kind == "render_clip" || kind == "revise_clip"
}

// testCancellation is the product's rule as the queue's own tests state it: the redacted
// kinds are the cancellable ones, and their handlers commit their own terminal writes.
type testCancellation struct{}

func (testCancellation) Kind(kind string) bool           { return redactedTestKind(kind) }
func (testCancellation) Allowed(kind string, _ int) bool { return redactedTestKind(kind) }

// reportingQueue is a queue with no store: the failure and log helpers under test need
// only the reporting collaborator.
func reportingQueue() *Queue { return &Queue{reporting: testReporting{}} }
