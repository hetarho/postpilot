package main

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/voice"
)

// jobKinds is what the queue's store is told about the work it holds. Clip work exists as
// a row while its owner is still deciding whether to pay for it, so nothing may pick it
// up until they approve (Deferred); an owner may stop any of it (Cancellable); and the
// two charged kinds authorize every model call against that stop (Authorized). The store
// is given the lists, so its SQL names no product.
func jobKinds() jobstore.Kinds {
	return jobstore.Kinds{
		Deferred:    []string{clip.JobKindGenerate, clip.JobKindRender, clip.JobKindRevise},
		Cancellable: []string{clip.JobKindGenerate, clip.JobKindRender, clip.JobKindRevise},
		Authorized:  []string{clip.JobKindGenerate, clip.JobKindRevise},
	}
}

// approvedCeilingKinds is the work the ledger may not start without an approved credit
// ceiling: a clip generation and an owner's revision each quote their whole run before
// the first model call (CLIP-19, CLIP-132). A render spends nothing and never reserves.
func approvedCeilingKinds() []string {
	return []string{clip.JobKindGenerate, clip.JobKindRevise}
}

// clipCancellation is the rule the queue asks before it accepts a stop: a render may
// always be stopped because it spends nothing, and charged clip work only under the
// cancellation policy this build honours and the owner approved.
type clipCancellation struct{}

func (clipCancellation) Kind(kind string) bool { return clip.IsJobKind(kind) }

func (clipCancellation) Allowed(kind string, cancellationPolicyVersion int) bool {
	if kind == clip.JobKindRender {
		return true
	}
	return clip.ChargedJobKind(kind) && cancellationPolicyVersion == clip.CancellationPolicyVersion
}

// voiceOwnedKind identifies personalization work whose writes are serialized per voice,
// even when the job also points at the post or source that caused the work. It lives
// beside the adapters that enqueue that work: the queue is told which guards to run, it
// does not know which kind belongs to whom.
func voiceOwnedKind(kind string) bool {
	switch kind {
	case job.KindAnalyzeVoice, job.KindLearnVoice, job.KindCompareVoiceRule, job.KindValidateVoiceProfile, job.KindSeedVoice:
		return true
	default:
		return false
	}
}

// postContentWork identifies the work that writes a post's content or observations when it
// completes: generation, revision and a model comparison, whose editor-origin verdict may apply
// its winner. The post context cannot see a comparison's origin, so every comparison counts. It
// is what makes saving a published address wait (post.ErrPostBusy), while work that only learns
// from a post — voice learning, memory extraction, a rule comparison — does not.
func postContentWork(kind string) bool {
	switch kind {
	case job.KindGenerate, job.KindRevise, job.KindModelExperiment:
		return true
	default:
		return false
	}
}

// postVoiceWork states what a post/voice job is attached to and which conflicts refuse
// it, in the order the queue checks them: one job at a time per post for its owner, and
// voice-owned work additionally one per (voice, kind). A voice-owned job that also names
// the post that caused it satisfies both.
func postVoiceWork(kind, userID, postSlug, voiceID string) ([]job.Subject, []job.Guard) {
	var subjects []job.Subject
	var guards []job.Guard
	if postSlug != "" {
		subject := job.Subject{Dimension: post.JobSubject, ID: postSlug}
		subjects = append(subjects, subject)
		guards = append(guards, job.Guard{Subject: subject, Filter: job.Filter{UserID: userID}})
	}
	if voiceID != "" {
		subject := job.Subject{Dimension: voice.JobSubject, ID: voiceID}
		subjects = append(subjects, subject)
		if postSlug == "" || voiceOwnedKind(kind) {
			guards = append(guards, job.Guard{Subject: subject, Filter: job.Filter{Kind: kind}})
		}
	}
	return subjects, guards
}
