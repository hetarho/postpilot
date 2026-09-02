// Package job owns durable long-running work and the in-process worker that consumes it.
package job

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	KindGenerate             = "generate"
	KindRevise               = "revise"
	KindAnalyzeVoice         = "analyze_voice"
	KindModelExperiment      = "model_experiment"
	KindLearnVoice           = "learn_voice"
	KindCompareVoiceRule     = "compare_voice_rule"
	KindValidateVoiceProfile = "validate_voice_profile"
	KindSeedVoice            = "seed_voice"

	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// voiceOwnedKind identifies personalization work whose writes are serialized per voice,
// even when the job also points at the post or source that caused the work.
func voiceOwnedKind(kind string) bool {
	switch kind {
	case KindAnalyzeVoice, KindLearnVoice, KindCompareVoiceRule, KindValidateVoiceProfile, KindSeedVoice:
		return true
	default:
		return false
	}
}

var (
	ErrNotFound       = errors.New("job not found")
	ErrForbidden      = errors.New("job belongs to another user")
	ErrActiveConflict = errors.New("an active job already exists")
	ErrInvalidTarget  = errors.New("job target does not belong to user")
	// ErrVoiceUnavailable closes the lifecycle race between a service's active-voice
	// precheck and the durable insert. The database is the final arbiter.
	ErrVoiceUnavailable = errors.New("job voice is deleted or unknown")
)

// ErrAlreadyInProgress identifies the durable job a caller should attach to.
type ErrAlreadyInProgress struct {
	ActiveID string
}

func (e *ErrAlreadyInProgress) Error() string {
	return fmt.Sprintf("job %s is already in progress", e.ActiveID)
}

func (e *ErrAlreadyInProgress) Unwrap() error { return ErrActiveConflict }

// NewJob is the immutable input recorded before the worker is woken. VoiceID is the voice
// the work was started for, frozen here so the handler can recheck it when it finally runs
// and so voice-owned kinds are guarded per voice rather than per account. The job context
// only carries the id; it never reads voice tables.
type NewJob struct {
	Kind           string
	UserID         string
	PostSlug       *string
	VoiceID        string
	ObserveModel   string
	WriteModel     string
	TargetLanguage string
	Payload        []byte
	// ExtraModels are refs the job will run that no stage column records — the two
	// candidates of an A/B comparison. They exist for the admission gate and are not
	// persisted: the experiment aggregate is where they are durable.
	ExtraModels []string
}

// models are the refs this job will actually run, for the admission gate. Both stage
// choices are explicit inputs ([I3]); an empty one means the kind does not use that stage.
func (n NewJob) models() []string {
	refs := make([]string, 0, 2+len(n.ExtraModels))
	for _, ref := range append([]string{n.ObserveModel, n.WriteModel}, n.ExtraModels...) {
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

// Job is the worker-facing record, including the kind-specific payload.
type Job struct {
	ID             string
	Kind           string
	UserID         string
	PostSlug       *string
	VoiceID        string
	Status         string
	Stage          string
	ProgressDone   int
	ProgressTotal  int
	Failure        *Failure
	ObserveModel   string
	WriteModel     string
	TargetLanguage string
	Payload        []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
}

// JobSummary is the public view returned to other contexts and the RPC edge.
type JobSummary struct {
	ID             string
	Kind           string
	UserID         string
	PostSlug       *string
	VoiceID        string
	Status         string
	Stage          string
	ProgressDone   int
	ProgressTotal  int
	Failure        *Failure
	ObserveModel   string
	WriteModel     string
	TargetLanguage string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func summarize(found Job) *JobSummary {
	return &JobSummary{
		ID: found.ID, Kind: found.Kind, UserID: found.UserID, PostSlug: found.PostSlug, VoiceID: found.VoiceID,
		Status: found.Status, Stage: found.Stage, ProgressDone: found.ProgressDone,
		ProgressTotal: found.ProgressTotal, Failure: cloneFailure(found.Failure),
		ObserveModel: found.ObserveModel, WriteModel: found.WriteModel, TargetLanguage: found.TargetLanguage,
		CreatedAt: found.CreatedAt, UpdatedAt: found.UpdatedAt,
	}
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("job: cannot read random bytes for a job id: " + err.Error())
	}
	return hex.EncodeToString(buf)
}
