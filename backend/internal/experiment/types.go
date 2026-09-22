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

// Origin is where a comparison was started, frozen at start. It decides the verdict form
// the review offers (MODEL-31, MODEL-36), and it is not the address the review was opened
// from (MODEL-60).
type Origin string

const (
	// OriginEditor: the editor's A/B comparison generation. The post it writes has no
	// content until one side is applied, so its verdict applies the winner.
	OriginEditor Origin = "editor"
	// OriginLab: the model lab. The verdict is a ranking pick that applies nothing; its
	// applications are separate follow-ups gated by the source post's status.
	OriginLab Origin = "lab"
)

// Window is how far back a leaderboard reads. There is no all-time value: an unbounded
// board ranks today's models by last year's verdicts (MODEL-38).
type Window string

const (
	WindowDay   Window = "day"
	WindowWeek  Window = "week"
	WindowMonth Window = "month"
)

// Length is the window measured back from the moment of the request.
func (w Window) Length() time.Duration {
	switch w {
	case WindowDay:
		return LeaderboardWindowDay
	case WindowMonth:
		return LeaderboardWindowMonth
	default:
		return LeaderboardWindowWeek
	}
}

func ParseWindow(value string) (Window, error) {
	switch Window(value) {
	case WindowDay, WindowWeek, WindowMonth:
		return Window(value), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidWindow, value)
	}
}

// Scope is whose verdicts a leaderboard replays. ScopeAll is a flag, never an account id:
// it aggregates every account's verdicts into per-model figures and names no account,
// experiment, output or note (MODEL-41).
type Scope string

const (
	ScopeMe  Scope = "me"
	ScopeAll Scope = "all"
)

func ParseScope(value string) (Scope, error) {
	switch Scope(value) {
	case ScopeMe, ScopeAll:
		return Scope(value), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidScope, value)
	}
}

// PostStatusFinalized is the one post status this context reacts to, mirrored as a plain
// string because the domain imports no other context's types (ARCH-7). The adapter that
// implements PostDirectory is what keeps the two spellings in step.
const PostStatusFinalized = "finalized"

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
	// Badges and OtherNote are verdict metadata: written once with the verdict, immutable
	// afterwards, and never weighed into a rating (MODEL-63). They are revealed with the
	// candidate's identity, never before it.
	Badges    []Badge
	OtherNote string

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
	TemplateName      string
	TargetLanguage    *Language
	Stage             Stage
	Origin            Origin
	Status            Status
	JobID             string
	InputSnapshot     []byte
	InputHash         string
	PromptVersion     string
	WinnerCandidateID string
	Outcome           Outcome
	ApplyFailure      *Failure
	// ApplyRequested records that this verdict owes a content application, so a failed one
	// keeps the comparison unresolved for its post. An editor verdict always owes one; a lab
	// pick owes one only once its separate application follow-up is taken.
	ApplyRequested    bool
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

// AppliesOnVerdict reports whether recording this comparison's verdict also applies its
// winner. Only the editor's write comparison does: a lab pick applies nothing (MODEL-36).
func (e Experiment) AppliesOnVerdict() bool {
	return e.Stage == StageWrite && e.Origin == OriginEditor
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
	// TemplateName is the 템플릿 the same frozen input carries, by name. Empty when the post had
	// none. It is a name rather than an id so the detail keeps reading correctly after the
	// template is renamed or deleted.
	TemplateName string
	// TargetLanguage is required for write snapshots and absent for observe/analyze.
	TargetLanguage *Language
}

type StartRequest struct {
	UserID   string
	PostSlug string
	// VoiceID is required for an analyze comparison and ignored otherwise: a write or
	// observe comparison takes its voice from the post.
	VoiceID string
	Stage   Stage
	// Origin is honoured for a write comparison only; observe and analyze can only be
	// started in the lab. An empty value means the editor, which is what every caller
	// predating the field was.
	Origin       Origin
	ObserveModel ModelRef
	ModelA       ModelRef
	ModelB       ModelRef
	TargetLength *int
	// ObserveFiles is the re-observation picker's answer for a write comparison, passed
	// straight through to the generation context's snapshot, which owns the reuse rule.
	// Presence is the contract: nil observes every attached photo, non-nil-but-empty
	// observes none. Ignored for observe and analyze comparisons.
	ObserveFiles *[]string
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

// VideoUnsupportedError preserves a pre-enqueue input refusal across the
// generation adapter without importing another context's domain into this one.
type VideoUnsupportedError struct{ Model string }

func (e *VideoUnsupportedError) Error() string { return ErrVideoUnsupported.Error() + ": " + e.Model }
func (e *VideoUnsupportedError) Unwrap() error { return ErrVideoUnsupported }

func (e *JobAlreadyInProgressError) Error() string {
	return "experiment job already in progress: " + e.ActiveID
}

var (
	ErrNotFound              = errors.New("experiment not found")
	ErrForbidden             = errors.New("experiment belongs to another user")
	ErrInvalidStage          = errors.New("invalid experiment stage")
	ErrModelRequired         = errors.New("two enabled suitable models are required")
	ErrVideoUnsupported      = errors.New("the observe model cannot read signed post video URLs")
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
	ErrPostFinalized         = errors.New("a finalized post cannot take a comparison result")
	ErrInvalidWindow         = errors.New("invalid leaderboard window")
	ErrInvalidScope          = errors.New("invalid leaderboard scope")
	ErrBadgesInvalid         = errors.New("the badges offered with this verdict are not ones it can carry")
)
