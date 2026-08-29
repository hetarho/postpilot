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
	Error        string
	Usage        Usage
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type Experiment struct {
	ID                string
	UserID            string
	PostSlug          string
	Stage             Stage
	Status            Status
	JobID             string
	InputSnapshot     []byte
	InputHash         string
	PromptVersion     string
	WinnerCandidateID string
	Outcome           Outcome
	ApplyError        string
	AppliedAt         *time.Time
	AdoptionRequested bool
	AdoptionError     string
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
	Ref                 ModelRef
	Label               string
	Vision              bool
	Enabled             bool
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
}

type StartRequest struct {
	UserID       string
	PostSlug     string
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
	ExperimentID string
	Stage        Stage
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
	ErrInvalidState          = errors.New("experiment state does not allow this operation")
	ErrCandidateNotFound     = errors.New("candidate not found")
	ErrConfirmationRequired  = errors.New("styleguide overwrite confirmation is required")
	ErrSnapshotUnavailable   = errors.New("입력이 변경되어 같은 조건으로 재시도할 수 없어요. 새 비교를 시작해 주세요.")
	ErrRetryModelUnavailable = errors.New("비교 모델을 더 이상 사용할 수 없어 새 비교를 시작해야 해요.")
)
