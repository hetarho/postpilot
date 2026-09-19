package main

import (
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/voice"
)

// deferredDispatchKinds are the kinds whose dispatch waits for an explicit activation:
// clip work exists as a row while its owner is still deciding whether to pay for it, and
// nothing may pick it up until they approve. The queue is told the list; it does not know
// why these kinds wait.
func deferredDispatchKinds() []string {
	return []string{job.KindGenerateClip, job.KindRenderClip, job.KindReviseClip}
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
