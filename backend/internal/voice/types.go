// Package voice owns an account's voices: each is an independent writing profile with its
// own samples, versions, rules and evidence, and a post names exactly one of them.
package voice

import (
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

const AnalysisJobKind = "analyze_voice"

const (
	LearnJobKind           = "learn_voice"
	CompareRuleJobKind     = "compare_voice_rule"
	ValidateProfileJobKind = "validate_voice_profile"
	SeedJobKind            = "seed_voice"
)

// DefaultVoiceName is the name of an account's first voice — created by migration 0009 for
// existing accounts and by the adduser bootstrap for new ones. The frontend renders the
// server value rather than repeating it.
const DefaultVoiceName = "기본 말투"

// VoiceNameMaxChars bounds a display name in Unicode scalar values; the frontend mirrors it
// in shared/config for early feedback, but this is the authoritative check.
const VoiceNameMaxChars = 50

// VoiceDescriptionMaxChars bounds the optional creation-time description in Unicode scalar
// values, mirrored in shared/config the same way. It is deliberately far below a sample's
// length: a description states a wanted register, it does not demonstrate one.
const VoiceDescriptionMaxChars = 500

var (
	ErrAnalyzeModelRequired = errors.New("an enabled analyze model is required")
	ErrSampleNotFound       = errors.New("voice sample not found")
	// ErrVersionSampleNotFound means this version never produced a post, which is an ordinary
	// state for a version rather than a failure.
	ErrVersionSampleNotFound = errors.New("voice version sample not found")
	ErrSampleMutation        = errors.New("voice sample change could not schedule analysis")
	ErrLearningNotFound      = errors.New("voice learning event not found")
	ErrRuleNotFound          = errors.New("voice rule not found")
	ErrConfirmationNotFound  = errors.New("voice rule confirmation not found")
	ErrComparisonNotFound    = errors.New("voice rule comparison not found")
	ErrValidationNotFound    = errors.New("voice profile validation not found")
	ErrInsufficientSources   = errors.New("at least three finalized authored posts are required")
	ErrInvalidLifecycle      = errors.New("invalid voice lifecycle transition")
	ErrPostNotFound          = errors.New("post not found")
	ErrForbidden             = errors.New("forbidden")

	ErrVoiceRequired = errors.New("a voice is required")
	// ErrVoiceNotFound covers unknown AND foreign ids on purpose: a voice that belongs to
	// another account is indistinguishable from one that does not exist.
	ErrVoiceNotFound  = errors.New("voice not found")
	ErrVoiceDeleted   = errors.New("voice is deleted")
	ErrVoiceNameTaken = errors.New("an active voice already has that name")
	ErrVoiceIsDefault = errors.New("the default voice cannot be deleted")
	// ErrVoiceBusy refuses a soft delete while a job, comparison, validation or analyze
	// experiment could still publish into the voice.
	ErrVoiceBusy = errors.New("voice has unfinished work that could still publish to it")
	// ErrBaselineVoiceMismatch: the post's machine baseline was written under a voice the
	// post no longer belongs to, so the finalized text is not evidence about the current one.
	ErrBaselineVoiceMismatch = errors.New("the machine baseline was written under another voice")
	ErrLanguageRequired      = errors.New("a content language is required")
	ErrLanguageUnsupported   = errors.New("the content language is unsupported")
	// ErrContentLanguageMismatch protects every post-derived learning boundary: content
	// may teach only a voice whose immutable source language is the same.
	ErrContentLanguageMismatch = errors.New("post content language does not match voice source language")
)

// Language is the voice context's pure canonical source/target language. Conversion to
// proto enums and SQL tags stays at the context edges.
type Language string

const (
	LanguageKorean  Language = "ko"
	LanguageEnglish Language = "en"
)

func ParseLanguage(value string) (Language, error) {
	language := Language(value)
	if !language.Valid() {
		return "", fmt.Errorf("%w: %q", ErrLanguageRequired, value)
	}
	return language, nil
}

func (l Language) Valid() bool { return l == LanguageKorean || l == LanguageEnglish }

// ContentLanguageMismatchError carries only canonical language tags. They are safe for
// the RPC edge to expose as localized-message parameters; authored content and ids never
// enter this error.
type ContentLanguageMismatchError struct {
	ContentLanguage Language
	SourceLanguage  Language
}

type InsufficientSourcesError struct{ Minimum int }

func (e *InsufficientSourcesError) Error() string { return ErrInsufficientSources.Error() }
func (e *InsufficientSourcesError) Unwrap() error { return ErrInsufficientSources }

func (e *ContentLanguageMismatchError) Error() string {
	return fmt.Sprintf("%v: content=%q source=%q", ErrContentLanguageMismatch, e.ContentLanguage, e.SourceLanguage)
}

func (e *ContentLanguageMismatchError) Unwrap() error { return ErrContentLanguageMismatch }

type SampleTooShortError struct{ Chars int }

func (e *SampleTooShortError) Error() string {
	return fmt.Sprintf("sample has %d characters; at least %d are required", e.Chars, SampleMinChars)
}

// VoiceNameError is an empty (after trimming) or over-long display name.
type VoiceNameError struct{ Chars int }

func (e *VoiceNameError) Error() string {
	if e.Chars == 0 {
		return "voice name is required"
	}
	return fmt.Sprintf("voice name has %d characters; at most %d are allowed", e.Chars, VoiceNameMaxChars)
}

// VoiceSeed is CreateVoice's optional described-voice request. A nil seed is the plain
// creation the product always had: an empty isolated profile and no provider work at all.
type VoiceSeed struct {
	Description  string
	AnalyzeModel llm.ModelRef
}

// VoiceDescriptionTooLongError is an over-long creation-time description. An empty one is
// not an error: the description is optional and its absence simply skips seeding.
type VoiceDescriptionTooLongError struct{ Chars int }

func (e *VoiceDescriptionTooLongError) Error() string {
	return fmt.Sprintf("voice description has %d characters; at most %d are allowed", e.Chars, VoiceDescriptionMaxChars)
}

// Voice is the aggregate root the directory manages. DeletedAt is a tombstone: the voice
// keeps its profile and its posts and stays readable, but cannot start or receive AI work.
type Voice struct {
	ID             string
	UserID         string
	Name           string
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	SourceLanguage Language
}

func (v Voice) Deleted() bool { return v.DeletedAt != nil }

type Sample struct {
	ID        string
	UserID    string
	VoiceID   string
	Label     string
	Body      string
	Chars     int
	CreatedAt time.Time
}

type Profile struct {
	UserID  string
	VoiceID string
	Voice   Voice
	// Rules is the refine step's "save as rule" text. It is no longer editable and no longer
	// reaches any RPC (change 16); the only writer is AppendRule and the only reader is the
	// prompt projection.
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
	AverageSentenceWords            *float64
	SentenceLength, ConnectiveStyle VoiceValue
	PreferredConnectives            []string
	Nominalization, PassiveTendency VoiceValue
}
type StructureProfile struct {
	IntroPattern, ClosingPattern                 VoiceValue
	ParagraphSentencesMin, ParagraphSentencesMax int
	HeadingHabit, ListHabit, EmojiUse            VoiceValue
}

// Each axis is a pointer so presence survives the round trip: an axis the analysis never
// answered is nil (published as unknown), not an indistinguishable neutral 0. A stored `0`
// in an older snapshot still decodes as present-0, so historical versions keep showing what
// they published.
type AxesProfile struct{ Involvement, Narrativity, PersuasionOvertness, Abstractness, AddresseeFocus, Humor *int }

// AxisValues lists the six axes in their canonical order with their JSON keys.
func (a AxesProfile) AxisValues() []struct {
	Key   string
	Value *int
} {
	return []struct {
		Key   string
		Value *int
	}{{"involvement", a.Involvement}, {"narrativity", a.Narrativity}, {"persuasion_overtness", a.PersuasionOvertness}, {"abstractness", a.Abstractness}, {"addressee_focus", a.AddresseeFocus}, {"humor", a.Humor}}
}

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
	ID, UserID, VoiceID, Statement, CanonicalKey string
	Layer                                        RuleLayer
	EvidenceCount                                int
	Status                                       RuleStatus
	Origin                                       string
	CreatedAt, LastEvidenceAt                    time.Time
}
type AuthoredSource struct {
	ID, UserID, VoiceID, PostSlug, LearningEventID, Title string
	Tags                                                  []string
	Body, Excerpt, EmbeddingRef                           string
	CreatedAt                                             time.Time
	SourceLanguage                                        Language
}
type Feedback struct {
	ID, UserID, VoiceID, PostSlug, SentenceRef, Kind, Reason, PayloadRef, ProcessingState string
	CreatedAt                                                                             time.Time
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
	ID, UserID, VoiceID string
	Version             int64
	Profile             StructuredProfile
	Origin              string
	RestoredFromVersion int64
	CreatedAt           time.Time
	// HasSample says whether this version can be PREVIEWED, without the list carrying every
	// post body the voice ever produced (change 16). The snapshot itself is fetched per
	// version, on open.
	HasSample bool
}

// VersionSample is a copy of the raw AI output of the last post generated under one profile
// version — the material that lets a version be read before it is adopted (change 16).
//
// Content is OPAQUE TEXT here and everywhere inside this context. Voice records what a profile
// version produced; it does not learn the shape of a post's content, so nothing in this package
// parses this string (ARCHITECTURE section 2, the anti-corruption boundary). It is a COPY, not a
// reference: deleting the source post, regenerating it, editing it by hand or reassigning it to
// another voice leaves the snapshot alone.
type VersionSample struct {
	UserID, VoiceID string
	Version         int64
	Content         string
	CreatedAt       time.Time
}
type ManualOverride struct {
	UserID, VoiceID string
	Layer           RuleLayer
	Field, Value    string
	UpdatedAt       time.Time
}

type PersonalizationConfig struct {
	FewShotTargetCount, FewShotMax, FewShotExcerptTargetChars, FewShotExcerptMaxChars int
	EmbeddingSwitchPosts, DiffMaxRules, DiffMinPatternEdits, RuleActivationEvidence   int
	RuleRetireAfter                                                                   time.Duration
	ValidationPostCount, EndingMaxConsecutive                                         int
}

// FinalizationInput is the post context's ownership-checked hand-off. VoiceID is the post's
// current voice and BaselineVoiceID the one its machine baseline was written under; learning
// requires them to agree so a correction of voice A's text is never read as evidence about B.
type FinalizationInput struct {
	PostSlug, UserID, VoiceID, BaselineVoiceID, BaselineJSON, FinalJSON, Title string
	Tags                                                                       []string
	BaselineRevision, ContentRevision                                          int64
	TargetLength                                                               *int
	ContentLanguage, VoiceSourceLanguage                                       Language
}

// LearningEvent freezes VoiceID at finalization; a retry follows the event, not the post's
// later assignment.
type LearningEvent struct {
	ID, UserID, VoiceID, PostSlug                                      string
	BaselineRevision                                                   int64
	InputHash, BaselineJSON, FinalJSON, ModelRef, Status, JobID, Error string
	CreatedAt                                                          time.Time
	ProcessedAt                                                        *time.Time
	ContentLanguage, SourceLanguage                                    Language
	Failure                                                            *Failure
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
	ID, UserID, VoiceID, RuleID, ExistingStatement, ProposedStatement, EventID string
	Status                                                                     string
	CreatedAt                                                                  time.Time
	ResolvedAt                                                                 *time.Time
}

type RuleComparison struct {
	ID, UserID, VoiceID, RuleID, SourceID                string
	ProfileVersion                                       int64
	ModelRef                                             string
	TargetLength                                         *int
	InputSnapshot, RuleOnSide, Status, JobID, ChosenSide string
	Candidates                                           []ComparisonCandidate
	CreatedAt                                            time.Time
	DecidedAt                                            *time.Time
	ActivationEvidence                                   int
	ProfileAfterDecision                                 *StructuredProfile
	SourceLanguage                                       Language
}
type ComparisonCandidate struct {
	ID, ComparisonID, DisplaySide, Output, Status, Error string
	Failure                                              *Failure
}
type ProfileValidation struct {
	ID, UserID, VoiceID            string
	ProfileVersion                 int64
	AnalyzeModelRef, WriteModelRef string
	JudgeEnabled                   bool
	Status, JobID                  string
	YCount, TotalCount             int
	Items                          []ValidationItem
	CreatedAt                      time.Time
	FinishedAt                     *time.Time
	SourceLanguage                 Language
}
type ValidationItem struct {
	ID, ValidationID, SourceID                                       string
	Position                                                         int
	Original, NeutralSummary, Regenerated, ScoresJSON, Status, Error string
	Failure                                                          *Failure
}

type AnalysisJob struct {
	UserID     string
	VoiceID    string
	WriteModel string
}

type AnalysisJobRequest struct {
	UserID     string
	VoiceID    string
	WriteModel string
}

// PersonalizationJobRequest freezes the owning voice on every provider-backed job so the
// queue guards per voice and a handler can recheck eligibility when it finally runs.
type PersonalizationJobRequest struct {
	Kind, UserID, VoiceID, PostSlug, Model, Payload string
	// ExtraModels are refs the job will run besides Model — profile validation also runs an
	// analyze model. The gate sees every ref the work will actually spend on, not just the
	// one the job row records.
	ExtraModels []string
	// CallCounts is how many times a ref runs, for the refs that run more than once. A
	// profile validation repeats its stages once per sampled post, so pricing it at one
	// call each would hold a fraction of what it spends.
	CallCounts map[string]int
}

type ActiveJob struct{ ID string }

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("analysis job %s is already in progress", e.ActiveID)
}
