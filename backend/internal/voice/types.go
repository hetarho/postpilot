// Package voice owns an account's voices: each is an independent writing profile with its
// own samples and versions, and a post names exactly one of them.
package voice

import (
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

const AnalysisJobKind = "analyze_voice"

const SeedJobKind = "seed_voice"

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
	// ErrLearningNotFound is a profile version the voice never published.
	ErrLearningNotFound = errors.New("voice profile version not found")
	ErrInvalidLifecycle = errors.New("invalid voice lifecycle transition")

	ErrVoiceRequired = errors.New("a voice is required")
	// ErrVoiceNotFound covers unknown AND foreign ids on purpose: a voice that belongs to
	// another account is indistinguishable from one that does not exist.
	ErrVoiceNotFound  = errors.New("voice not found")
	ErrVoiceDeleted   = errors.New("voice is deleted")
	ErrVoiceNameTaken = errors.New("an active voice already has that name")
	ErrVoiceIsDefault = errors.New("the default voice cannot be deleted")
	// ErrVoiceBusy refuses a soft delete while a job could still publish into the voice.
	ErrVoiceBusy           = errors.New("voice has unfinished work that could still publish to it")
	ErrLanguageRequired    = errors.New("a content language is required")
	ErrLanguageUnsupported = errors.New("the content language is unsupported")
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
	UserID      string
	VoiceID     string
	Voice       Voice
	UpdatedAt   time.Time
	Samples     []Sample
	ActiveJobID string
	Structured  StructuredProfile
	Versions    []ProfileVersion
	// SeedFailure is why the seeding a described creation started failed, kept while the
	// voice still has no published version (VOICE-19): the 말투 tab says so after the job ends
	// and after a reload, until 기존 글 가져오기 publishes one.
	SeedFailure *Failure
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
	// OverrideBase holds, per overridden field ("layer.field"), the value it held before the
	// override replaced it, so clearing the override can return it (VOICE-28).
	OverrideBase map[string]VoiceValue `json:",omitempty"`
}
type ProfileVersion struct {
	ID, UserID, VoiceID string
	Version             int64
	Profile             StructuredProfile
	Origin              string
	RestoredFromVersion int64
	CreatedAt           time.Time
	// HasSample says whether this version can be PREVIEWED, without the list carrying every
	// post body the voice ever produced (VOICE-29). The snapshot itself is fetched per
	// version, on open.
	HasSample bool
}

// VersionSample is a copy of the raw AI output of the last post generated under one profile
// version — the material that lets a version be read before it is adopted (VOICE-29).
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
	FewShotMax, FewShotExcerptTargetChars, FewShotExcerptMaxChars int
	EndingMaxConsecutive                                          int
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
	Kind, UserID, VoiceID, Model, Payload string
}

type ActiveJob struct{ ID string }

// FinishedJob is the latest job of a kind, terminal or not, as the profile reads it.
type FinishedJob struct {
	ID, Status string
	Failure    *Failure
}

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("analysis job %s is already in progress", e.ActiveID)
}
