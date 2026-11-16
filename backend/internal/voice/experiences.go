package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// VoiceExperienceNameMaxChars bounds an experience's optional name in Unicode scalar values
// (VOICE-78); the frontend reads the thread shape from the prompts, not from this constant.
const VoiceExperienceNameMaxChars = 40

var (
	// ErrExperienceKindUnknown is a kind outside the six top-level kinds (VOICE-78).
	ErrExperienceKindUnknown = errors.New("unknown experience kind")
	// ErrExperienceNotFound is an experience id that is unknown, belongs to another voice or
	// to another account — indistinguishable on purpose (VOICE-3).
	ErrExperienceNotFound = errors.New("experience not found")
	// ErrExperienceMismatch is an answer whose prompt and experience do not fit: a thread
	// question without an experience, one of another kind, or a situation given one.
	ErrExperienceMismatch = errors.New("the prompt does not match the experience")
)

// ExperienceNameError is an over-long experience name; an empty name is valid.
type ExperienceNameError struct{ Chars int }

func (e *ExperienceNameError) Error() string {
	return fmt.Sprintf("experience name has %d characters; at most %d are allowed", e.Chars, VoiceExperienceNameMaxChars)
}

// ExperienceStore is the experiences of a voice and the thread answers keyed by them
// (VOICE-78).
type ExperienceStore interface {
	InsertExperience(ctx context.Context, e Experience) error
	// ListExperiences is the voice's experiences, newest first.
	ListExperiences(ctx context.Context, userID, voiceID string) ([]Experience, error)
	// GetExperience is nil for an unknown or foreign id.
	GetExperience(ctx context.Context, userID, voiceID, experienceID string) (*Experience, error)
	// AnswerExperiencePrompt writes or replaces the answer of one (experience, question) and
	// reports false when the experience is not this owner's on this voice.
	AnswerExperiencePrompt(ctx context.Context, sample Sample) (bool, error)
	GetExperienceAnswer(ctx context.Context, userID, voiceID, experienceID, promptKey string) (*Sample, error)
}

func validExperienceKind(kind ExperienceKind) bool {
	for _, info := range experienceKinds {
		if info.Kind == kind {
			return true
		}
	}
	return false
}

// StartExperience begins one experience of the owner's the learning chat asks about: a kind and
// an optional name, trimmed and bounded. A deleted voice refuses before anything is written
// (VOICE-15); an unknown or foreign voice is not found (VOICE-3). It calls no model.
func (s *Service) StartExperience(ctx context.Context, userID, voiceID string, kind ExperienceKind, name string) (Experience, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return Experience{}, err
	}
	if !validExperienceKind(kind) {
		return Experience{}, ErrExperienceKindUnknown
	}
	name = strings.TrimSpace(name)
	if chars := utf8.RuneCountInString(name); chars > VoiceExperienceNameMaxChars {
		return Experience{}, &ExperienceNameError{Chars: chars}
	}
	e := Experience{ID: s.newID(), UserID: userID, VoiceID: voiceID, Kind: kind, Name: name, CreatedAt: s.now()}
	if err := s.experiences.InsertExperience(ctx, e); err != nil {
		return Experience{}, fmt.Errorf("insert experience: %w", err)
	}
	return e, nil
}

// ListExperiences is the voice's experiences, newest first; a deleted voice stays readable.
func (s *Service) ListExperiences(ctx context.Context, userID, voiceID string) ([]Experience, error) {
	if _, err := s.ownedVoice(ctx, userID, voiceID); err != nil {
		return nil, err
	}
	list, err := s.experiences.ListExperiences(ctx, userID, voiceID)
	if err != nil {
		return nil, fmt.Errorf("list experiences: %w", err)
	}
	return list, nil
}

// answerThread saves a thread question's answer under its experience (VOICE-78): the experience
// must be this owner's on this voice and of the prompt's kind, and replying again replaces only
// that (experience, prompt) answer, keeping its row. The same text changes nothing.
func (s *Service) answerThread(ctx context.Context, userID, voiceID string, prompt Prompt, experienceID, body string) (Sample, error) {
	if prompt.Kind == "" || experienceID == "" {
		return Sample{}, ErrExperienceMismatch
	}
	found, err := s.experiences.GetExperience(ctx, userID, voiceID, experienceID)
	if err != nil {
		return Sample{}, fmt.Errorf("get experience: %w", err)
	}
	if found == nil {
		return Sample{}, ErrExperienceNotFound
	}
	if found.Kind != prompt.Kind {
		return Sample{}, ErrExperienceMismatch
	}
	previous, err := s.experiences.GetExperienceAnswer(ctx, userID, voiceID, experienceID, prompt.Key)
	if err != nil {
		return Sample{}, fmt.Errorf("get experience answer: %w", err)
	}
	if previous != nil && previous.Body == body {
		return *previous, nil
	}
	sample := Sample{
		ID: s.newID(), UserID: userID, VoiceID: voiceID, Kind: SampleKindAnswer, PromptKey: prompt.Key,
		Body: body, Chars: utf8.RuneCountInString(body), ContentRevision: 1, ExperienceID: experienceID, CreatedAt: s.now(),
	}
	if previous != nil {
		sample.ID, sample.ContentRevision = previous.ID, previous.ContentRevision+1
	}
	saved, err := s.experiences.AnswerExperiencePrompt(ctx, sample)
	if err != nil {
		return Sample{}, fmt.Errorf("answer experience prompt: %w", err)
	}
	if !saved {
		return Sample{}, ErrExperienceNotFound
	}
	return sample, nil
}
