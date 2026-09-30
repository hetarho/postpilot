package experiment

import (
	"context"
	"time"
)

// JobSubject is what the experiment context calls itself when it addresses the job
// queue. An experiment carries no column of its own: its id is the job payload, which
// the schema surfaces as a generated column for this lookup.
const JobSubject = "model_experiment"

// RunLedger is one experiment as a whole: minting it, finding it again, and what the queue
// and the post screen ask about it.
type RunLedger interface {
	Create(ctx context.Context, found Experiment) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (Experiment, error)
	// List is the account's comparisons newest first; an empty stage or source is every one.
	List(ctx context.Context, userID string, stage Stage, source Source) ([]Experiment, error)
	PendingForPost(ctx context.Context, userID, postSlug string) (*Experiment, error)
	BlockingWriteForPost(ctx context.Context, userID, postSlug string) (string, error)
	SetJob(ctx context.Context, id, userID, jobID string) error
	SetSnapshot(ctx context.Context, id string, snapshot Snapshot, hash string) error
	SetStatus(ctx context.Context, id string, status Status, finishedAt *time.Time) error
	ListQueued(ctx context.Context) ([]string, error)
}

// CandidateLedger is the two sides of a run while they are being produced, including what a
// restart has to make good.
type CandidateLedger interface {
	StartCandidate(ctx context.Context, experimentID, candidateID string, now time.Time) error
	CompleteCandidate(ctx context.Context, candidate Candidate) error
	FailUnfinished(ctx context.Context, experimentID string, failure Failure, now time.Time) error
	RecoverInterrupted(ctx context.Context, failure Failure, now time.Time) (int64, error)
	ResetFailedCandidates(ctx context.Context, experimentID string) (int64, error)
	RestoreFailedCandidates(ctx context.Context, experimentID string, candidates []Candidate) error
}

// OutcomeLedger is what the owner decided and what happened when it was applied.
type OutcomeLedger interface {
	// Decide writes the verdict and the badges that explain it in one transaction: a board
	// that counted a badge whose verdict was never recorded would be counting nothing.
	Decide(ctx context.Context, id, userID, candidateID string, status Status, outcome Outcome, applyRequested, adoptionRequested bool, badges []CandidateBadges, decidedAt, expiresAt time.Time) (bool, error)
	// SetApplyRequested records that a decided verdict now owes a content application, so a
	// failure leaves the comparison unresolved for its post. Idempotent.
	SetApplyRequested(ctx context.Context, id, userID string) error
	SetApplyFailure(ctx context.Context, id, userID string, failure Failure) error
	SetApplied(ctx context.Context, id, userID string, now time.Time) error
	// SetAdoptionRequested records that a decided verdict now owes an adoption (MODEL-36).
	// Idempotent.
	SetAdoptionRequested(ctx context.Context, id, userID string) error
	SetAdoptionFailure(ctx context.Context, id, userID string, failure Failure) error
	SetAdopted(ctx context.Context, id, userID string, now time.Time) error
	// LeaderboardData returns the winner verdicts decided at or after `since` and the call
	// accounting of the comparisons resolved in the same span. `userID` is honoured only for
	// ScopeMe; ScopeAll reads every account and the caller's id never reaches the rows.
	LeaderboardData(ctx context.Context, userID string, stage Stage, since time.Time, scope Scope) ([]Experiment, []Candidate, []BadgeTally, error)
}

// RunRetention is the cleanup: a run that has outlived its window, and one whose post is
// gone.
type RunRetention interface {
	PurgeExpired(ctx context.Context, before time.Time) (int64, error)
	PurgePost(ctx context.Context, userID, postSlug string) error
}

// Storage is every behaviour the experiment context's SQL store happens to implement: the
// composition root's handle, not a port (ARCH-6).
type Storage interface {
	RunLedger
	CandidateLedger
	OutcomeLedger
	RunRetention
}

// PostDirectory is the post context's published status of one owned post, consumed before a
// lab comparison's content application: a finalized post's confirmed content is not rewritten
// from a comparison (MODEL-37). The composition root adapts it; this context never reads post
// tables.
type PostDirectory interface {
	Status(ctx context.Context, userID, slug string) (string, error)
}

// VoiceDirectory is the voice context's published check that a voice is owned and alive,
// consumed before any retry that would run in a voice's name. The
// composition root adapts it; this context never reads voice tables.
type VoiceDirectory interface {
	ActiveVoice(ctx context.Context, userID, voiceID string) error
}

// VoiceReflection is the voice context's published 말투 반영 비교 behaviour (MODEL-67): it freezes
// one voice's input, runs one candidate over that snapshot, reads the prompt and the answer back
// for the review, and measures a piece against the voice. The composition root adapts it; this
// context never reads voice tables.
type VoiceReflection interface {
	Snapshot(ctx context.Context, userID, voiceID, promptKey string) (ReflectionSnapshot, error)
	Run(ctx context.Context, content []byte, model ModelRef) (CandidateResult, error)
	PromptText(promptKey string) string
	Answer(content []byte) (string, error)
	Compare(ctx context.Context, userID, voiceID, text string) ([]ItemComparison, error)
}

type Catalog interface {
	Resolve(ref ModelRef) (Model, bool)
	Adopt(ctx context.Context, userID string, stage Stage, ref ModelRef) error
	Active(ctx context.Context, userID string, stage Stage) (ModelRef, bool, error)
	// Recommended is every ref a current recommendation set names for the stage.
	Recommended(ctx context.Context, stage Stage) ([]ModelRef, error)
}

type Jobs interface {
	EnqueueExperiment(ctx context.Context, request JobRequest) (string, error)
	HasRunnableExperiment(ctx context.Context, experimentID string) (bool, error)
}

type Runner interface {
	Snapshot(ctx context.Context, request StartRequest) (Snapshot, error)
	PrepareWrite(ctx context.Context, found Experiment, progress Progress) (Snapshot, error)
	RunCandidate(ctx context.Context, found Experiment, candidate Candidate, progress Progress) (CandidateResult, error)
	ApplyWinner(ctx context.Context, found Experiment, candidate Candidate) error
}

type Progress func(stage string, done, total int)
