// Package experiment owns blind pairwise model comparisons, their verdicts, private
// leaderboards, and retention. Domain types intentionally carry no wire/SQL tags.
package experiment

import (
	"errors"
	"fmt"
	"time"
)

type Stage string

const (
	StageObserve Stage = "observe"
	StageAnalyze Stage = "analyze"
	StageWrite   Stage = "write"
)

func ParseStage(value string) (Stage, error) {
	switch Stage(value) {
	case StageObserve, StageAnalyze, StageWrite:
		return Stage(value), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidStage, value)
	}
}

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusReview    Status = "review"
	StatusPartial   Status = "partial"
	StatusDecided   Status = "decided"
	StatusDismissed Status = "dismissed"
	StatusFailed    Status = "failed"
)

type CandidateStatus string

const (
	CandidatePending   CandidateStatus = "pending"
	CandidateRunning   CandidateStatus = "running"
	CandidateSucceeded CandidateStatus = "succeeded"
	CandidateFailed    CandidateStatus = "failed"
)

type DisplaySide string

const (
	SideLeft  DisplaySide = "left"
	SideRight DisplaySide = "right"
)

type Outcome string

const (
	OutcomeWinner   Outcome = "winner"
	OutcomeSkipped  Outcome = "skipped"
	OutcomeUnpaired Outcome = "unpaired"
)

type CostSource string

const (
	CostReported    CostSource = "reported"
	CostEstimated   CostSource = "estimated"
	CostUnavailable CostSource = "unavailable"
	CostMixed       CostSource = "mixed"
)

type ModelRef struct {
	ProviderID string
	ModelID    string
}

func (r ModelRef) String() string { return r.ProviderID + "/" + r.ModelID }

type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
	CostMicrousd     int64
	CostSource       CostSource
	LatencyMS        int64
}

type Candidate struct {
	ID           string
	ExperimentID string
	Model        ModelRef
	ModelLabel   string
	DisplaySide  DisplaySide
	Status       CandidateStatus
	Output       []byte
	Failure      *Failure
	Usage        Usage
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

// Experiment.VoiceID is frozen at start: the voice whose corpus an analyze comparison read,
// or the voice the compared post was in for a write one. A winner may only ever be applied
// back to that same voice.
type Experiment struct {
	ID                string
	UserID            string
	PostSlug          string
	VoiceID           string
	PurposeName       string
	TargetLanguage    *Language
	Stage             Stage
	Status            Status
	JobID             string
	InputSnapshot     []byte
	InputHash         string
	PromptVersion     string
	WinnerCandidateID string
	Outcome           Outcome
	ApplyFailure      *Failure
	AppliedAt         *time.Time
	AdoptionRequested bool
	AdoptionFailure   *Failure
	AdoptedAt         *time.Time
	CreatedAt         time.Time
	FinishedAt        *time.Time
	DecidedAt         *time.Time
	ContentExpiresAt  *time.Time
	Candidates        []Candidate
}

func (e Experiment) Revealed() bool {
	return e.Status == StatusDecided || e.Status == StatusDismissed
}

func (e Experiment) Winner() *Candidate {
	for i := range e.Candidates {
		if e.Candidates[i].ID == e.WinnerCandidateID {
			return &e.Candidates[i]
		}
	}
	return nil
}

type Model struct {
	Ref     ModelRef
	Label   string
	Vision  bool
	Enabled bool
	// Stages this model is registered to serve (change 20), in the same strings Stage uses.
	Stages              []string
	InputUSDPerMillion  string
	OutputUSDPerMillion string
}

type CandidateResult struct {
	Output []byte
	Usage  UsageReport
}

type UsageReport struct {
	PromptTokens     int64
	CompletionTokens int64
	CostMicrousd     int64
	CostReported     bool
}

type Snapshot struct {
	Content       []byte
	PromptVersion string
	// VoiceID is the voice the runner froze the input for; the aggregate records it.
	VoiceID string
	// PurposeName is the 용도 the same frozen input carries, by name. Empty when the post had
	// none. It is a name rather than an id so the detail keeps reading correctly after the
	// purpose is renamed or deleted.
	PurposeName string
	// TargetLanguage is required for write snapshots and absent for observe/analyze.
	TargetLanguage *Language
}

type StartRequest struct {
	UserID   string
	PostSlug string
	// VoiceID is required for an analyze comparison and ignored otherwise: a write or
	// observe comparison takes its voice from the post.
	VoiceID      string
	Stage        Stage
	ObserveModel ModelRef
	ModelA       ModelRef
	ModelB       ModelRef
	TargetLength *int
}

type StartResult struct {
	ExperimentID string
	JobID        string
}

type JobRequest struct {
	UserID       string
	PostSlug     string
	VoiceID      string
	ExperimentID string
	Stage        Stage
	// TargetLanguage is frozen for write jobs and absent for observe/analyze jobs.
	TargetLanguage *Language
	// Models are the two candidate refs this comparison will run. The enqueue seam gates
	// them against the caller's plan; one comparison still consumes exactly one admission.
	Models []string
}

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return "experiment job already in progress: " + e.ActiveID
}

var (
	ErrNotFound              = errors.New("experiment not found")
	ErrForbidden             = errors.New("experiment belongs to another user")
	ErrInvalidStage          = errors.New("invalid experiment stage")
	ErrModelRequired         = errors.New("two enabled suitable models are required")
	ErrDuplicateCandidates   = errors.New("comparison candidates must differ")
	ErrInvalidTargetLength   = errors.New("target length must be positive")
	ErrLanguageRequired      = errors.New("a supported write target language is required")
	ErrInvalidState          = errors.New("experiment state does not allow this operation")
	ErrCandidateNotFound     = errors.New("candidate not found")
	ErrConfirmationRequired  = errors.New("styleguide overwrite confirmation is required")
	ErrSnapshotUnavailable   = errors.New("experiment snapshot is unavailable")
	ErrRetryModelUnavailable = errors.New("experiment retry model is unavailable")
	ErrVoiceRequired         = errors.New("an active voice is required to compare analyze models")
	ErrVoiceUnavailable      = errors.New("the voice this comparison belongs to is deleted or unknown")
)
