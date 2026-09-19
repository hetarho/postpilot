package job_test

import (
	"errors"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
)

// reportingForTest is the shape the composition root wires (cmd/api jobReporting): the
// queue itself knows none of this, so its tests hand it the same collaborator the product
// does — a provider's failure vocabulary and diagnostics, and one redacted kind family.
type reportingForTest struct{}

func (reportingForTest) Failure(err error) job.Failure {
	normalized := llm.NormalizeFailure(err)
	var detailed interface{ Failure() llm.Failure }
	if errors.As(err, &detailed) {
		normalized = detailed.Failure()
	}
	return job.Failure{Reason: normalized.Reason, Params: normalized.Params, TechnicalDetail: normalized.TechnicalDetail}
}

func (reportingForTest) LogAttrs(err error) []any {
	diagnostic, ok := llm.DiagnosticOf(err)
	if !ok {
		return nil
	}
	return []any{"operation", diagnostic.Operation, "error_class", diagnostic.Class}
}

func (reportingForTest) SafeStage(kind, stage string) (string, bool) {
	if kind != deferredKind {
		return "", false
	}
	switch stage {
	case "queued", "prepare", "analyze", "render", "save":
		return stage, true
	}
	return "unknown", true
}

func (reportingForTest) Redacted(kind string) bool { return kind == deferredKind }

// The dimensions the product's contexts use for their own subjects. They are spelled out
// here rather than imported so the queue's tests stay as product-free as the queue is.
const (
	postSubject        = "post"
	voiceSubject       = "voice"
	clipProjectSubject = "clip_project"
)

// attach and clipJob mirror the composition root's subject mapping (cmd/api
// `postVoiceWork` / the clip jobs adapter) so these tests still enqueue the guards the
// product runs. The mapping itself is unit-tested where it lives, in cmd/api.
func attach(in job.NewJob, slug, voiceID string) job.NewJob {
	if slug != "" {
		subject := job.Subject{Dimension: postSubject, ID: slug}
		in.Subjects = append(in.Subjects, subject)
		in.Guards = append(in.Guards, job.Guard{Subject: subject, Filter: job.Filter{UserID: in.UserID}})
	}
	if voiceID != "" {
		subject := job.Subject{Dimension: voiceSubject, ID: voiceID}
		in.Subjects = append(in.Subjects, subject)
		if slug == "" || voiceOwnedTestKind(in.Kind) {
			in.Guards = append(in.Guards, job.Guard{Subject: subject, Filter: job.Filter{Kind: in.Kind}})
		}
	}
	return in
}

// clipJob mirrors the clip adapter: charged clip work defers its hold until its owner
// approves the quote, and a render spends nothing at all.
func clipJob(in job.NewJob, project string) job.NewJob {
	in.DeferHold = in.Kind == "generate_clip" || in.Kind == "revise_clip"
	in.NonMetered = in.Kind == "render_clip"
	subject := job.Subject{Dimension: clipProjectSubject, ID: project}
	in.Subjects = append(in.Subjects, subject)
	in.Guards = append(in.Guards, job.Guard{Subject: subject, Filter: job.Filter{UserID: in.UserID}})
	return in
}

func voiceOwnedTestKind(kind string) bool {
	switch kind {
	case job.KindAnalyzeVoice, job.KindLearnVoice, job.KindCompareVoiceRule, job.KindValidateVoiceProfile, job.KindSeedVoice:
		return true
	default:
		return false
	}
}

// jobKindsForTest is the same shape the composition root wires: work that waits for its
// owner's approval before dispatch, may be cancelled, and authorizes each model call.
func jobKindsForTest() jobstore.Kinds {
	return jobstore.Kinds{
		Deferred:    []string{deferredKind},
		Cancellable: []string{deferredKind},
		Authorized:  []string{deferredKind},
	}
}

// deferredKind is a made-up kind: the queue's tests need work that waits for an
// activation, and nothing about that behaviour is a product's.
const deferredKind = "deferred-work"
