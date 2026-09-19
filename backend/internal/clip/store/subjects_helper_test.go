package store_test

import (
	"errors"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/voice"
)

// attach and clipJob mirror the composition root's subject mapping (cmd/api
// `postVoiceWork` / the clip jobs adapter) so these tests still enqueue the guards the
// product runs. The mapping itself is unit-tested where it lives, in cmd/api.
func attach(in job.NewJob, slug, voiceID string) job.NewJob {
	if slug != "" {
		subject := job.Subject{Dimension: post.JobSubject, ID: slug}
		in.Subjects = append(in.Subjects, subject)
		in.Guards = append(in.Guards, job.Guard{Subject: subject, Filter: job.Filter{UserID: in.UserID}})
	}
	if voiceID != "" {
		subject := job.Subject{Dimension: voice.JobSubject, ID: voiceID}
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
	subject := job.Subject{Dimension: clip.JobSubject, ID: project}
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

// deferredKindsForTest is the same list the composition root wires.

// jobKindsForTest is the same shape the composition root wires.
func jobKindsForTest() jobstore.Kinds {
	return jobstore.Kinds{
		Deferred:    []string{clip.JobKindGenerate, clip.JobKindRender, clip.JobKindRevise},
		Cancellable: []string{clip.JobKindGenerate, clip.JobKindRender, clip.JobKindRevise},
		Authorized:  []string{clip.JobKindGenerate, clip.JobKindRevise},
	}
}

// jobReportingForTest gives the queue the provider vocabulary the product wires.
type jobReportingForTestType struct{}

func (jobReportingForTestType) Failure(err error) job.Failure {
	normalized := llm.NormalizeFailure(err)
	var detailed interface{ Failure() llm.Failure }
	if errors.As(err, &detailed) {
		normalized = detailed.Failure()
	}
	return job.Failure{Reason: normalized.Reason, Params: normalized.Params, TechnicalDetail: normalized.TechnicalDetail}
}
func (jobReportingForTestType) LogAttrs(error) []any { return nil }
func (jobReportingForTestType) SafeStage(kind, stage string) (string, bool) {
	if !clip.IsJobKind(kind) {
		return "", false
	}
	return clip.SafeJobStage(stage), true
}
func (jobReportingForTestType) Redacted(kind string) bool { return clip.IsJobKind(kind) }

func jobReportingForTest() job.Reporting { return jobReportingForTestType{} }

// clipCancellationForTest is the product's cancellation rule.
type clipCancellationForTest struct{}

func (clipCancellationForTest) Kind(kind string) bool { return clip.IsJobKind(kind) }
func (clipCancellationForTest) Allowed(kind string, version int) bool {
	if kind == clip.JobKindRender {
		return true
	}
	return clip.ChargedJobKind(kind) && version == clip.CancellationPolicyVersion
}
