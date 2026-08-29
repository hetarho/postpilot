// Package voice owns one account's editable writing profile and its source samples.
package voice

import (
	"errors"
	"fmt"
	"time"
)

const AnalysisJobKind = "analyze_voice"

const (
	LearnJobKind           = "learn_voice"
	CompareRuleJobKind     = "compare_voice_rule"
	ValidateProfileJobKind = "validate_voice_profile"
)

var (
	ErrAnalyzeModelRequired = errors.New("an enabled analyze model is required")
	ErrSampleNotFound       = errors.New("voice sample not found")
	ErrSampleMutation       = errors.New("voice sample change could not schedule analysis")
	ErrLearningNotFound     = errors.New("voice learning event not found")
	ErrRuleNotFound         = errors.New("voice rule not found")
	ErrConfirmationNotFound = errors.New("voice rule confirmation not found")
	ErrComparisonNotFound   = errors.New("voice rule comparison not found")
	ErrValidationNotFound   = errors.New("voice profile validation not found")
	ErrInsufficientSources  = errors.New("at least three finalized authored posts are required")
	ErrInvalidLifecycle     = errors.New("invalid voice lifecycle transition")
	ErrPostNotFound         = errors.New("post not found")
	ErrForbidden            = errors.New("forbidden")
)

type SampleTooShortError struct{ Chars int }

func (e *SampleTooShortError) Error() string {
	return fmt.Sprintf("sample has %d characters; at least %d are required", e.Chars, SampleMinChars)
}

type Sample struct {
	ID        string
	UserID    string
	Label     string
	Body      string
	Chars     int
	CreatedAt time.Time
}

type Profile struct {
	UserID      string
	Styleguide  string
	Rules       string
	UpdatedAt   time.Time
	Samples     []Sample
	ActiveJobID string
	Structured  StructuredProfile
	Versions    []ProfileVersion
	SourceCount int
	CanValidate bool
}

type ValueSource string

const (
	SourceUnknown  ValueSource = "unknown"
	SourceMeasured ValueSource = "measured"
	SourceAnalyzed ValueSource = "analyzed"
	SourceManual   ValueSource = "manual"
)

type VoiceValue struct {
	Value   string
	Source  ValueSource
	Unknown bool
}
type WeightedWord struct {
	Word         string
	Alternatives []string
	Weight       int
}
type BannedItem struct {
	Value  string
	Reason string
}
type EndingRatio struct {
	Ending string
	Ratio  float64
}
type LexicalProfile struct {
	PreferredWords              []WeightedWord
	BannedWords, BannedPatterns []BannedItem
	Description                 VoiceValue
}
type EndingsProfile struct {
	BaseRegister                                 VoiceValue
	Distribution                                 []EndingRatio
	BannedEndings, SignatureEndings, Constraints []string
}
type SyntaxProfile struct {
	AverageSentenceChars            float64
	SentenceLength, ConnectiveStyle VoiceValue
	PreferredConnectives            []string
	Nominalization, PassiveTendency VoiceValue
}
type StructureProfile struct {
	IntroPattern, ClosingPattern                 VoiceValue
	ParagraphSentencesMin, ParagraphSentencesMax int
	HeadingHabit, ListHabit, EmojiUse            VoiceValue
}
type AxesProfile struct{ Involvement, Narrativity, PersuasionOvertness, Abstractness, AddresseeFocus, Humor int }

type RuleLayer string

const (
	LayerLexical   RuleLayer = "lexical"
	LayerEndings   RuleLayer = "endings"
	LayerSyntax    RuleLayer = "syntax"
	LayerStructure RuleLayer = "structure"
	LayerAxes      RuleLayer = "axes"
)

type RuleStatus string

const (
	RuleCandidate RuleStatus = "candidate"
	RuleActive    RuleStatus = "active"
	RuleRetired   RuleStatus = "retired"
	RuleRejected  RuleStatus = "rejected"
)

type ContrastRule struct {
	ID, UserID, Statement, CanonicalKey string
	Layer                               RuleLayer
	EvidenceCount                       int
	Status                              RuleStatus
	Origin                              string
	CreatedAt, LastEvidenceAt           time.Time
}
type AuthoredSource struct {
	ID, UserID, PostSlug, LearningEventID, Title string
	Tags                                         []string
	Body, Excerpt, EmbeddingRef                  string
	CreatedAt                                    time.Time
}
type Feedback struct {
	ID, UserID, PostSlug, SentenceRef, Kind, Reason, PayloadRef, ProcessingState string
	CreatedAt                                                                    time.Time
}
type StructuredProfile struct {
	Version     int64
	UpdatedAt   time.Time
	SourceCount int
	Empty       bool
	Lexical     LexicalProfile
	Endings     EndingsProfile
	Syntax      SyntaxProfile
	Structure   StructureProfile
	Axes        AxesProfile
	Rules       []ContrastRule
	Sources     []AuthoredSource
	Feedback    []Feedback
}
type ProfileVersion struct {
	ID, UserID          string
	Version             int64
	Profile             StructuredProfile
	Origin              string
	RestoredFromVersion int64
	CreatedAt           time.Time
}
type ManualOverride struct {
	UserID       string
	Layer        RuleLayer
	Field, Value string
	UpdatedAt    time.Time
}

type PersonalizationConfig struct {
	FewShotTargetCount, FewShotMax, FewShotExcerptTargetChars, FewShotExcerptMaxChars int
	EmbeddingSwitchPosts, DiffMaxRules, DiffMinPatternEdits, RuleActivationEvidence   int
	RuleRetireAfter                                                                   time.Duration
	ValidationPostCount, EndingMaxConsecutive                                         int
}

type FinalizationInput struct {
	PostSlug, UserID, BaselineJSON, FinalJSON, Title string
	Tags                                             []string
	BaselineRevision, ContentRevision                int64
	TargetLength                                     int
}
type LearningEvent struct {
	ID, UserID, PostSlug                                               string
	BaselineRevision                                                   int64
	InputHash, BaselineJSON, FinalJSON, ModelRef, Status, JobID, Error string
	CreatedAt                                                          time.Time
	ProcessedAt                                                        *time.Time
}
type LearningJob struct{ UserID, EventID, WriteModel string }
type LearningResult struct {
	Source  AuthoredSource
	Profile StructuredProfile
	Rules   []ExtractedRule
}
type ExtractedRule struct {
	Statement         string
	Layer             RuleLayer
	Citations         []string
	MatchRuleID       string
	ContradictsRuleID string
}

type RuleConfirmation struct {
	ID, UserID, RuleID, ExistingStatement, ProposedStatement, EventID string
	Status                                                            string
	CreatedAt                                                         time.Time
	ResolvedAt                                                        *time.Time
}

type RuleComparison struct {
	ID, UserID, RuleID, SourceID                         string
	ProfileVersion                                       int64
	ModelRef                                             string
	TargetLength                                         int
	InputSnapshot, RuleOnSide, Status, JobID, ChosenSide string
	Candidates                                           []ComparisonCandidate
	CreatedAt                                            time.Time
	DecidedAt                                            *time.Time
	ActivationEvidence                                   int
	ProfileAfterDecision                                 *StructuredProfile
}
type ComparisonCandidate struct{ ID, ComparisonID, DisplaySide, Output, Status, Error string }
type ProfileValidation struct {
	ID, UserID                     string
	ProfileVersion                 int64
	AnalyzeModelRef, WriteModelRef string
	JudgeEnabled                   bool
	Status, JobID                  string
	YCount, TotalCount             int
	Items                          []ValidationItem
	CreatedAt                      time.Time
	FinishedAt                     *time.Time
}
type ValidationItem struct {
	ID, ValidationID, SourceID                                       string
	Position                                                         int
	Original, NeutralSummary, Regenerated, ScoresJSON, Status, Error string
}

type AnalysisJob struct {
	UserID     string
	WriteModel string
}

type AnalysisJobRequest struct {
	UserID     string
	WriteModel string
}

type PersonalizationJobRequest struct{ Kind, UserID, PostSlug, Model, Payload string }

type ActiveJob struct{ ID string }

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("analysis job %s is already in progress", e.ActiveID)
}
