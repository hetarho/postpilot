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
	// ErrVoiceNotReady is 말투 만들기 or 다시 분석 below 100% (VOICE-32).
	ErrVoiceNotReady = errors.New("the voice's 학습 글 are not enough yet")
	// ErrPromptNotFound is a prompt key the shared set does not hold.
	ErrPromptNotFound = errors.New("voice prompt not found")
	// ErrPromptAnswered is a second answer to a prompt that holds one (VOICE-60).
	ErrPromptAnswered = errors.New("the prompt already holds an answer")
	// ErrAnswerRequired is an empty answer.
	ErrAnswerRequired = errors.New("an answer is required")
	// ErrPhotoRequired is a photo prompt answered without an uploaded photo.
	ErrPhotoRequired = errors.New("a photo prompt needs an uploaded photo")
	// ErrInvalidPhoto is a photo whose size or dimensions a post photo could not have either.
	ErrInvalidPhoto = errors.New("invalid voice photo")
	// ErrNoPreviousAnalysis is 이전 분석으로 되돌리기 with nothing to return to (VOICE-30).
	ErrNoPreviousAnalysis = errors.New("the voice has no previous analysis")
	ErrInvalidLifecycle   = errors.New("invalid voice lifecycle transition")
	// ErrPostNotFound and ErrPostForbidden are a post PostContents does not know and one that
	// belongs to another account.
	ErrPostNotFound  = errors.New("post not found")
	ErrPostForbidden = errors.New("post belongs to another account")

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
	// ReadinessPercent is the meter's share until the voice is made (VOICE-9); 0 once made.
	ReadinessPercent int
}

func (v Voice) Deleted() bool { return v.DeletedAt != nil }

// SampleKind is what a 학습 글 is: a post the owner wrote by hand and pasted, or an answer to
// one of the shared prompts (VOICE-59).
type SampleKind string

const (
	SampleKindPost   SampleKind = "post"
	SampleKindAnswer SampleKind = "answer"
)

// Sample is one 학습 글. An answer names its prompt, and a photo prompt's answer the private
// photo it was written on (VOICE-60); a post has neither. Label is empty for an answer: its
// prompt text is product copy, never a row.
type Sample struct {
	ID          string
	UserID      string
	VoiceID     string
	Kind        SampleKind
	PromptKey   string
	Label       string
	Body        string
	Chars       int
	PhotoKey    string
	PhotoWidth  int
	PhotoHeight int
	CreatedAt   time.Time
}

// HasPhoto reports an answer written on a photo.
func (s Sample) HasPhoto() bool { return s.PhotoKey != "" }

// Title is how the analysis corpus heads a 학습 글: a post by its label, an answer by its prompt.
func (s Sample) Title() string {
	if s.Kind == SampleKindAnswer {
		if prompt, ok := PromptByKey(s.PromptKey); ok {
			return prompt.Text
		}
	}
	return s.Label
}

// PhotoUpload is a photo prompt's photo between its presign and its answer (VOICE-60).
type PhotoUpload struct {
	ID, UserID, VoiceID, PromptKey, Key string
	ExpiresAt, CreatedAt                time.Time
}

// Profile is a voice as its 말투 분석 tab and its 학습 글 tab read it.
type Profile struct {
	UserID      string
	VoiceID     string
	Voice       Voice
	Samples     []Sample
	ActiveJobID string
	// Readiness is the meter over every 학습 글 (VOICE-32); 다시 분석 needs it at 100% too.
	Readiness Readiness
	// Analysis is the current analysis, nil until the voice is made (VOICE-25).
	Analysis    *Analysis
	HasPrevious bool
	Notice      Notice
}

// NoticeKind is how the 학습 글 moved since the current analysis read them (VOICE-21).
type NoticeKind string

const (
	NoticeNone    NoticeKind = ""
	NoticeAdded   NoticeKind = "added"
	NoticeChanged NoticeKind = "changed"
)

// Notice is `새 학습 글 N편` (added) or `학습 글이 바뀌었어요` (changed).
type Notice struct {
	Kind  NoticeKind
	Count int
}

// AIField is which part of the AI's reading an example shows.
type AIField string

const (
	AIImpression       AIField = "impression"
	AITics             AIField = "tics"
	AISignaturePhrases AIField = "signature_phrases"
)

// Tic is a verbal tic and when it appears.
type Tic struct {
	Phrase string `json:"phrase"`
	When   string `json:"when"`
}

// AIExample is a sentence the AI cited, kept only when it occurs verbatim in a 학습 글.
type AIExample struct {
	Field      AIField `json:"field"`
	Sentence   string  `json:"sentence"`
	MaterialID string  `json:"material_id"`
}

// AIPart is what the analysis call writes: only what cannot be counted (VOICE-24).
type AIPart struct {
	Impression       string      `json:"impression"`
	Tics             []Tic       `json:"tics"`
	SignaturePhrases []string    `json:"signature_phrases"`
	Examples         []AIExample `json:"examples"`
}

// Analysis is one immutable snapshot (VOICE-26): the counted fingerprint, the AI part, the 학습
// 글 it read and when.
type Analysis struct {
	Counted      Fingerprint
	AI           AIPart
	MaterialIDs  []string
	AnalyzeModel string
	CreatedAt    time.Time
}

// AnalysisJob is one queued analysis. MaterialIDs is the snapshot frozen at its start
// (VOICE-22): the run reads those 학습 글 that still exist and nothing added since.
type AnalysisJob struct {
	UserID      string
	VoiceID     string
	WriteModel  string
	MaterialIDs []string
}

// AnalysisJobRequest starts one analysis over the snapshot its start read. PromptTokens is the
// size of the prompt the run will send over that snapshot, at one token per Unicode character,
// which the hold prices (QUOTA-14).
type AnalysisJobRequest struct {
	UserID       string
	VoiceID      string
	WriteModel   string
	MaterialIDs  []string
	PromptTokens int
}

type ActiveJob struct{ ID string }

type JobAlreadyInProgressError struct{ ActiveID string }

func (e *JobAlreadyInProgressError) Error() string {
	return fmt.Sprintf("analysis job %s is already in progress", e.ActiveID)
}
