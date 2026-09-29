package voice

import (
	"context"
	"fmt"
)

type PromptProfile struct {
	Styleguide     string
	Excerpts       []string
	Empty          bool
	TargetLanguage Language
	Portable       bool
	// Version is the published profile version this projection was read from, so a post
	// generated from it can be filed under it (VOICE-29).
	Version int64
}

// PromptProfileForLanguage publishes a deterministic full or portable projection for one
// concrete target. Voice owns the field selection so no consumer can accidentally leak
// Korean excerpts or lexical rules into another language.
func (s *Service) PromptProfileForLanguage(ctx context.Context, userID, voiceID string, target Language) (PromptProfile, error) {
	return s.PromptProfileForTopicAndLanguage(ctx, userID, voiceID, target, "", nil)
}

// The topic and tags ranked finalized-post excerpts, which are gone; the pasted samples are
// taken newest first whatever the topic.
func (s *Service) PromptProfileForTopicAndLanguage(ctx context.Context, userID, voiceID string, target Language, _ string, _ []string) (PromptProfile, error) {
	if !target.Valid() {
		return PromptProfile{}, ErrLanguageRequired
	}
	return s.promptProfileForTopic(ctx, userID, voiceID, target)
}

// PromptProfileForTopic projects exactly one voice for a Korean target. Every row it reads is
// keyed by that voice, so a well-trained sibling voice contributes nothing — an empty voice
// prompts as empty rather than borrowing. A deleted voice is refused: nothing may be written
// in it.
func (s *Service) PromptProfileForTopic(ctx context.Context, userID, voiceID, _ string, _ []string) (PromptProfile, error) {
	return s.promptProfileForTopic(ctx, userID, voiceID, LanguageKorean)
}

// promptProfileForTopic compares the target with Korean, the only language a voice has
// (VOICE-10): a Korean target gets the complete projection, any other the portable one.
func (s *Service) promptProfileForTopic(ctx context.Context, userID, voiceID string, target Language) (PromptProfile, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return PromptProfile{}, err
	}
	profile, err := s.profiles.GetProfile(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("get profile for prompt: %w", err)
	}
	if target != LanguageKorean {
		style := renderPortableProfile(profile.Structured)
		return PromptProfile{
			Styleguide: style, Empty: style == "", TargetLanguage: target, Portable: true, Version: profile.Structured.Version,
		}, nil
	}
	// The pasted samples, newest first, are the only excerpt source: one budget of
	// FewShotMax excerpts cut near the target length (VOICE-46).
	samples, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("list excerpts: %w", err)
	}
	excerpts := make([]string, 0, min(s.config.FewShotMax, len(samples)))
	for _, sample := range samples {
		if len(excerpts) >= s.config.FewShotMax {
			break
		}
		candidate := excerptAroundTarget(sample.Body, s.config.FewShotExcerptTargetChars, s.config.FewShotExcerptMaxChars)
		if !containsString(excerpts, candidate) {
			excerpts = append(excerpts, candidate)
		}
	}
	// ONE representation, injected ONCE. The analysis text reaches the model through the
	// structured profile's lexical description and nowhere else.
	style := renderStructuredProfile(profile.Structured)
	// An empty voice is exactly "nothing to learn from and nothing published": no samples and
	// no structured version.
	empty := len(samples) == 0 && profile.Structured.Version == 0
	return PromptProfile{Styleguide: style, Excerpts: excerpts, Empty: empty, TargetLanguage: target, Version: profile.Structured.Version}, nil
}
