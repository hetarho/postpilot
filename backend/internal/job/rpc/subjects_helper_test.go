package rpc_test

import "github.com/postpilot/backend/internal/job"

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

func clipJob(in job.NewJob, project string) job.NewJob {
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

// deferredKindsForTest is the same list the composition root wires.
func deferredKindsForTest() []string {
	return []string{job.KindGenerateClip, job.KindRenderClip, job.KindReviseClip}
}
