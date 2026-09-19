package main

import (
	"errors"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

// jobReporting is how the queue describes work it runs: the provider's own failure
// vocabulary, the diagnostics that may be logged with it, and the stage names and
// redaction rule of the one kind family whose errors can wrap user content.
//
// It lives here rather than in the queue because neither the provider's error classes nor
// a product's stage names are the queue's to know (ARCH-6, ARCH-9).
type jobReporting struct{}

func (jobReporting) Failure(err error) job.Failure {
	normalized := llm.NormalizeFailure(err)
	var detailed interface{ Failure() llm.Failure }
	if errors.As(err, &detailed) {
		normalized = detailed.Failure()
	}
	return job.Failure{Reason: normalized.Reason, Params: normalized.Params, TechnicalDetail: normalized.TechnicalDetail}
}

func (jobReporting) LogAttrs(err error) []any {
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

func (jobReporting) SafeStage(kind, stage string) (string, bool) {
	if !clip.IsJobKind(kind) {
		return "", false
	}
	return clip.SafeJobStage(stage), true
}

// Clip failures may wrap subprocess stderr, media paths or provider bodies.
func (jobReporting) Redacted(kind string) bool { return clip.IsJobKind(kind) }
