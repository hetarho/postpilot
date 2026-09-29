// Package voice owns an account's voices: each is an independent writing profile with its
// own samples and versions, and a post names at most one of them.
package voice

import (
	"errors"
	"fmt"
	"time"
)

const AnalysisJobKind = "analyze_voice"

// VoiceNameMaxChars bounds a display name in Unicode scalar values; the frontend mirrors it
// in entities/voice/config for early feedback, but this is the authoritative check.
const VoiceNameMaxChars = 50

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
	// ErrVoiceNotMade is a voice with no published analysis: it cannot be the 기본 (VOICE-32).
	ErrVoiceNotMade = errors.New("voice is not made yet")
	// ErrVoiceBusy refuses a soft delete while a job could still publish into the voice.
	ErrVoiceBusy = errors.New("voice has unfinished work that could still publish to it")
	// ErrLanguageRequired is a projection asked for no valid target language.
	ErrLanguageRequired = errors.New("a target language is required")
)

// Language is a projection's target language. A voice itself is Korean (VOICE-10): the
// target only decides whether the projection is complete or portable (VOICE-46).
type Language string

const (
	LanguageKorean  Language = "ko"
	LanguageEnglish Language = "en"
)

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

// Voice is the aggregate root the directory manages. DeletedAt is a tombstone: the voice
// keeps its profile and its posts and stays readable, but cannot start or receive AI work.
type Voice struct {
	ID        string
	UserID    string
	Name      string
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	// Made is whether the voice has a published analysis: only a made voice can be assigned
	// to a post or write one (POST-23).
	Made bool
	// SampleCount and AnalyzedAt are the directory row's meta line (VOICE-52): how many 학습
	// 글 the voice holds and when its current analysis was published (nil until made).
	SampleCount int
	AnalyzedAt  *time.Time
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

type ActiveJob struct{ ID string }

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("analysis job %s is already in progress", e.ActiveID)
}
