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
	StageWrite   Stage = "write"
)

func ParseStage(value string) (Stage, error) {
	switch Stage(value) {
	case StageObserve, StageWrite:
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

// Source is what a write comparison was drawn from: a post, or — as 말투 반영 비교 — one voice and
// one of its answered prompts (MODEL-30, MODEL-67). Its verdicts count on the write board
// either way.
type Source string

const (
	SourcePost  Source = "post"
	SourceVoice Source = "voice"
)

// ParseSource reads a history filter; empty is every source.
func ParseSource(value string) (Source, error) {
	switch Source(value) {
	case "", SourcePost, SourceVoice:
		return Source(value), nil
	default:
		return "", fmt.Errorf("%w: source %q", ErrInvalidStage, value)
	}
}

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

// The four post statuses this context reacts to, mirrored as plain strings because the domain
// imports no other context's types (ARCH-7). A draft or a post in revision takes a comparison's
// result; a finalized one takes only the editor's, which reopens it; a published one takes none
// (MODEL-37). The adapter that implements PostDirectory is what keeps the spellings in step.
const (
	PostStatusDraft     = "draft"
	PostStatusReview    = "review"
	PostStatusFinalized = "finalized"
	PostStatusPublished = "published"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusReview    Status = "review"
	StatusPartial   Status = "partial"
	StatusDecided   Status = "decided"
	StatusDismissed Status = "dismissed"
	StatusFailed    Status = "failed"
	StatusCompleted Status = "completed"
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
	SideC     DisplaySide = "c"
	SideD     DisplaySide = "d"
	SideE     DisplaySide = "e"
)

type ReviewMode string

const (
	ReviewPairwise         ReviewMode = "pairwise"
	ReviewCandidateRanking ReviewMode = "candidate_ranking"
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
	Rank      int

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

type CandidateRank struct {
	CandidateID string
	Rank        int
	Badges      []Badge
	OtherNote   string
}

// Experiment.VoiceID is frozen at start: the voice the compared post was in for a write
// comparison. A winner may only ever be applied back to that same voice.
type Experiment struct {
	ID       string
	UserID   string
	PostSlug string
	VoiceID  string
	// Source, and for a voice-sourced comparison the prompt and the answer it withheld; the
	// answer's text lives only in the snapshot (MODEL-42).
	Source             Source
	VoicePromptKey     string
	VoiceMaterialID    string
	TemplateName       string
	TargetLanguage     *Language
	Stage              Stage
	Origin             Origin
	ReviewMode         ReviewMode
	CompletedAt        *time.Time
	AppliedCandidateID string
	AdoptedCandidateID string
	Status             Status
	JobID              string
	InputSnapshot      []byte
	InputHash          string
	PromptVersion      string
	WinnerCandidateID  string
	Outcome            Outcome
	ApplyFailure       *Failure
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
	return e.Status == StatusDecided || e.Status == StatusDismissed || e.Status == StatusCompleted
}

func (e Experiment) Winner() *Candidate {
	for i := range e.Candidates {
		if e.Candidates[i].ID == e.WinnerCandidateID {
			return &e.Candidates[i]
		}
	}
	return nil
}

func (e Experiment) Candidate(id string) *Candidate {
	for i := range e.Candidates {
		if e.Candidates[i].ID == id {
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
	// Stages this model is registered to serve (MODEL-14), in the same strings Stage uses.
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
	// TargetLanguage is required for write snapshots and absent for observe.
	TargetLanguage *Language
}

type StartRequest struct {
	UserID   string
	PostSlug string
	Stage    Stage
	// Origin is honoured for a write comparison only; observe can only be started in the
	// lab. An empty value means the editor, which is what every caller
	// predating the field was.
	Origin       Origin
	ObserveModel ModelRef
	ModelA       ModelRef
	ModelB       ModelRef
	// Candidates is the full list for a new lab comparison. Empty preserves the legacy pair.
	Candidates   []ModelRef
	TargetLength *int
	// ObserveFiles is the re-observation picker's answer for a write comparison, passed
	// straight through to the generation context's snapshot, which owns the reuse rule.
	// Presence is the contract: nil observes every attached photo, non-nil-but-empty
	// observes none. Ignored for observe comparisons.
	ObserveFiles *[]string
}

// ReflectionStartRequest is 말투 반영 비교's start (MODEL-67): one voice, one of its answered
// prompts and the write pair.
type ReflectionStartRequest struct {
	UserID     string
	VoiceID    string
	PromptKey  string
	ModelA     ModelRef
	ModelB     ModelRef
	Candidates []ModelRef
}

// ReflectionSnapshot is the voice context's frozen 말투 반영 비교 input and what the comparison
// records beside it.
type ReflectionSnapshot struct {
	Content       []byte
	PromptVersion string
	PromptKey     string
	MaterialID    string
	// Photo is a photo prompt, which needs both candidates to read images.
	Photo bool
}

// ItemComparison is one counted item of a piece against its voice (VOICE-62), in the voice
// context's own words; the rpc edge puts it on the shared wire message.
type ItemComparison struct {
	Item     string
	Unknown  bool
	Distance float64
	Headline string
	Facets   []ComparisonFacet
}

// ComparisonFacet is one value of an item on both sides, in its unit: numbers, or terms.
type ComparisonFacet struct {
	Key                   string
	Unit                  string
	Voice, Text           float64
	VoiceTerms, TextTerms []string
}

// ReflectionDetail is what a 말투 반영 비교's review reads beside the pieces, computed on read: the
// prompt, the owner's answer while the snapshot keeps it, and each delivered piece measured
// against the voice's current analysis, by candidate id.
type ReflectionDetail struct {
	PromptText  string
	Answer      string
	Comparisons map[string][]ItemComparison
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
	// TargetLanguage is frozen for write jobs and absent for observe jobs.
	TargetLanguage *Language
	// ObserveModel is the shared preparation call for a post write comparison.
	ObserveModel string
	// ObserveCalls is how many observe calls one pass over the frozen input makes — every observe
	// candidate's, or a write comparison's shared preparation — as the runner counts them. Zero is
	// a real answer: a preparation that reuses every observation observes nothing.
	ObserveCalls int
	// Models are every candidate ref this comparison will run. The enqueue seam gates
	// them against the caller's plan; one comparison still consumes exactly one admission.
	Models []string
}

type JobAlreadyInProgressError struct{ ActiveID string }

// VideoUnsupportedError preserves a pre-enqueue input refusal across the
// generation adapter without importing another context's domain into this one.
type VideoUnsupportedError struct{ Model string }

// RequiredTemplateAnswerError keeps the first missing template field across the generation
// snapshot adapter, before this comparison creates a row or holds credits.
type RequiredTemplateAnswerError struct{ Label string }

func (e *RequiredTemplateAnswerError) Error() string {
	return "a required template answer is missing: " + e.Label
}

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
	ErrCandidateCount        = errors.New("comparison requires two to five candidates, or exactly two in the editor")
	ErrMixedCandidateForms   = errors.New("legacy pair and full candidate list cannot be combined")
	ErrInvalidTargetLength   = errors.New("target length must be positive")
	ErrLanguageRequired      = errors.New("a supported write target language is required")
	ErrInvalidState          = errors.New("experiment state does not allow this operation")
	ErrCandidateNotFound     = errors.New("candidate not found")
	ErrSnapshotUnavailable   = errors.New("experiment snapshot is unavailable")
	ErrRetryModelUnavailable = errors.New("experiment retry model is unavailable")
	ErrVoiceUnavailable      = errors.New("the voice this comparison belongs to is deleted")
	// ErrVoiceNotMade: the compared post's voice has no published analysis yet (GEN-25).
	ErrVoiceNotMade = errors.New("the compared post's voice is not made yet")
	// ErrVoiceNotFound is an unknown or foreign voice, which the owner cannot tell apart
	// (MODEL-31); a deleted one of theirs is ErrVoiceUnavailable.
	ErrVoiceNotFound = errors.New("the voice this comparison names is not found")
	ErrPostFinalized = errors.New("a finalized post cannot take a comparison result")
	ErrPostPublished = errors.New("a published post cannot take a comparison result")
	ErrInvalidWindow = errors.New("invalid leaderboard window")
	ErrInvalidScope  = errors.New("invalid leaderboard scope")
	ErrBadgesInvalid = errors.New("the badges offered with this verdict are not ones it can carry")
	ErrRanksInvalid  = errors.New("rank every successful candidate with dense positive ranks")
	// The 말투 반영 비교 refusals (MODEL-31, MODEL-67): no voice named, a prompt the shared set does
	// not hold or the voice has not answered, and a photo prompt either candidate cannot read.
	ErrVoiceRequired    = errors.New("a voice is required")
	ErrPromptNotFound   = errors.New("the prompt is not found")
	ErrPromptUnanswered = errors.New("the prompt has no answer to compare against")
	ErrPhotoUnsupported = errors.New("a photo prompt needs both candidates to read images")
)

// Writing tests coexist with retained paid comparisons; their mutations never reuse a ranking.
type TestFactor string

const (
	FactorModel     TestFactor = "model"
	FactorVoice     TestFactor = "voice"
	FactorTemplate  TestFactor = "template"
	FactorGuideline TestFactor = "guideline"
)

func (f TestFactor) Valid() bool {
	return f == FactorModel || f == FactorVoice || f == FactorTemplate || f == FactorGuideline
}

type TestStatus string

const (
	TestQueued    TestStatus = "queued"
	TestRunning   TestStatus = "running"
	TestPartial   TestStatus = "partial"
	TestReview    TestStatus = "review"
	TestCompleted TestStatus = "completed"
	TestCancelled TestStatus = "cancelled"
	TestFailed    TestStatus = "failed"
)

func (s TestStatus) Valid() bool {
	switch s {
	case TestQueued, TestRunning, TestPartial, TestReview, TestCompleted, TestCancelled, TestFailed:
		return true
	}
	return false
}
func ValidTestCount(n int) bool { return n == 2 || n == 4 || n == 8 || n == 16 }

type TestCandidateStatus string

const (
	TestCandidatePending   TestCandidateStatus = "pending"
	TestCandidateRunning   TestCandidateStatus = "running"
	TestCandidateSucceeded TestCandidateStatus = "succeeded"
	TestCandidateFailed    TestCandidateStatus = "failed"
	TestCandidateCancelled TestCandidateStatus = "cancelled"
)

func (s TestCandidateStatus) Valid() bool {
	switch s {
	case TestCandidatePending, TestCandidateRunning, TestCandidateSucceeded, TestCandidateFailed, TestCandidateCancelled:
		return true
	}
	return false
}

type TestPublicationAction string

const (
	TestSaveSetting TestPublicationAction = "save_setting"
	TestUseSetting  TestPublicationAction = "use_setting"
	TestAdoptModel  TestPublicationAction = "adopt_model"
	TestApplyOutput TestPublicationAction = "apply_output"
)

func (a TestPublicationAction) Valid() bool {
	return a == TestSaveSetting || a == TestUseSetting || a == TestAdoptModel || a == TestApplyOutput
}

type TestPublicationStatus string

const (
	TestPublicationPending   TestPublicationStatus = "pending"
	TestPublicationConfirmed TestPublicationStatus = "confirmed"
	TestPublicationConflict  TestPublicationStatus = "conflict"
)

func (s TestPublicationStatus) Valid() bool {
	return s == TestPublicationPending || s == TestPublicationConfirmed || s == TestPublicationConflict
}

type TestEntrantRef struct {
	SourceKind                               string
	Model                                    ModelRef
	SettingKind, SettingID, SettingRevision  string
	AuthoringSessionID, AuthoringCandidateID string
	AuthoringRevision                        uint32
}
type TestInput struct {
	SourcePostSlug                       string
	InputRevision, ContentRevision       int64
	Material                             string
	Fictional                            bool
	AttachmentIDs                        []string
	TemplateAnswers                      []TestAnswer
	ObserveModel, WriteModel             ModelRef
	VoiceID, TemplateID, GuidelineSlotID string
	TargetLanguage                       string
	TargetLength, TagCount               int
	UseMemory                            bool
	QualityRules                         []string
}
type TestAnswer struct {
	Label, Answer string
	Enabled       bool
}
type TestStart struct {
	UserID, RequestKey, QuoteKey string
	Factor                       TestFactor
	ModelStage                   Stage
	Count                        int
	Entrants                     []TestEntrantRef
	Input                        TestInput
}

var (
	ErrTestCount     = errors.New("writing test entrant count is invalid")
	ErrTestFactor    = errors.New("writing test factor is invalid")
	ErrTestEntrant   = errors.New("writing test entrant is invalid")
	ErrTestDuplicate = errors.New("writing test entrants repeat")
	ErrTestOperation = errors.New("writing test operation key is invalid")
)

// ValidateTestShape runs before resolution, job/hold creation or any provider work.
// Eligibility, semantic snapshot uniqueness and domain revisions are checked by owned ports.
func ValidateTestShape(r TestStart) error {
	if !ValidTestCount(r.Count) || len(r.Entrants) != r.Count {
		return ErrTestCount
	}
	if !r.Factor.Valid() || (r.Factor == FactorModel && r.ModelStage != StageObserve && r.ModelStage != StageWrite) || (r.Factor != FactorModel && r.ModelStage != "") {
		return ErrTestFactor
	}
	if r.UserID == "" || r.RequestKey == "" {
		return ErrTestOperation
	}
	seen := map[string]bool{}
	for _, e := range r.Entrants {
		var key string
		switch e.SourceKind {
		case "model":
			if r.Factor != FactorModel || e.Model.ProviderID == "" || e.Model.ModelID == "" || e.SettingID != "" || e.AuthoringSessionID != "" {
				return ErrTestEntrant
			}
			key = "model:" + e.Model.String()
		case "setting":
			if r.Factor == FactorModel || e.SettingID == "" || e.SettingRevision == "" || e.SettingKind != string(r.Factor) || e.Model.ProviderID != "" || e.Model.ModelID != "" || e.AuthoringSessionID != "" {
				return ErrTestEntrant
			}
			key = "setting:" + e.SettingKind + ":" + e.SettingID
		case "authoring_candidate":
			if r.Factor == FactorModel || e.AuthoringSessionID == "" || e.AuthoringCandidateID == "" || e.Model.ProviderID != "" || e.Model.ModelID != "" || e.SettingID != "" {
				return ErrTestEntrant
			}
			key = "authoring:" + e.AuthoringSessionID + ":" + e.AuthoringCandidateID
		default:
			return ErrTestEntrant
		}
		if seen[key] {
			return ErrTestDuplicate
		}
		seen[key] = true
	}
	return nil
}

type TestMutation struct {
	UserID, TestID, RequestKey string
	ExpectedRevision           uint32
}
type MatchDecision struct {
	TestMutation
	MatchID, WinnerCandidateID string
}
type TestCandidateIdentity struct {
	Label     string
	Ref       TestEntrantRef
	Synthetic bool
}
type TestCandidate struct {
	ID, TestID, UserID                string
	SeedPosition                      int
	Ref                               TestEntrantRef
	SourceRevision, SemanticKey       string
	FrozenVariant, Output, Accounting []byte
	Status                            string
	Failure                           *Failure
	Identity                          *TestCandidateIdentity
}

// BlindCandidate intentionally excludes source refs/revisions, seed positions, frozen values
// and accounting. A UI must use this projection rather than marshal the private store record.
type BlindCandidate struct {
	ID, DisplayLabel, Status string
	Output                   []byte
	Failure                  *Failure
	Identity                 *TestCandidateIdentity
}

func (c TestCandidate) Project(revealed bool, label string) BlindCandidate {
	out := BlindCandidate{ID: c.ID, DisplayLabel: label, Status: c.Status, Output: append([]byte(nil), c.Output...)}
	// Technical detail can reveal the model, so it never enters a blind projection.
	if c.Failure != nil {
		out.Failure = &Failure{Reason: c.Failure.Reason}
	}
	if revealed && c.Identity != nil {
		identity := *c.Identity
		out.Identity = &identity
	}
	return out
}

type TestMatch struct {
	ID                                     string
	Round, Index                           int
	LeftID, RightID, WinnerID, DecisionKey string
}
type TestPublication struct{ ID, UserID, TestID, WinnerID, Action, RequestKey, TargetID, Status, Fingerprint string }
type WritingTest struct {
	ID, UserID, Kind, SourcePostSlug, JobID, CommonHash, PromptVersion, WinnerID string
	Factor                                                                       TestFactor
	ModelStage                                                                   Stage
	Count                                                                        int
	Status                                                                       TestStatus
	Revision                                                                     uint32
	Input                                                                        TestInput
	CommonSnapshot                                                               []byte
	PurgeFence                                                                   uint64
	Candidates                                                                   []TestCandidate
	Matches                                                                      []TestMatch
	Publications                                                                 []TestPublication
	CreatedAt, UpdatedAt                                                         time.Time
}
type FrozenTestVariant struct {
	Reference             TestEntrantRef
	Content               []byte
	SemanticKey, Revision string
	Synthetic             bool
}
type TestSnapshot struct {
	Common                               []byte
	Hash, PromptVersion, AssignmentsHash string
	Variants                             []FrozenTestVariant
}
type TestCall struct {
	Ref                                   ModelRef
	Stage                                 Stage
	Count, PromptTokens, CompletionTokens int
}
type TestPlan struct {
	Snapshot        TestSnapshot
	Calls           []TestCall
	EstimateCredits int
	Free            bool
}
type OutputApplication struct {
	TestMutation
	WinnerID, PostSlug, AssignmentsHash string
	InputRevision, ContentRevision      int64
	Output, Storyline, Baseline         []byte
	ContentLanguage                     string
}
type ModelAdoption struct {
	TestMutation
	WinnerID string
	Stage    Stage
	Model    ModelRef
}
type WinnerPublication struct {
	TestMutation
	WinnerID, Action, Name, Scope string
	ScopeIDs                      []string
	MakeDefault                   bool
	Variant                       FrozenTestVariant
}
type PublicationReceipt struct {
	UserID, TestID, WinnerID, Action, RequestKey, TargetID string
	ResultingRevision                                      int64
}
