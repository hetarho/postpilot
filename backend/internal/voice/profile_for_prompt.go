package voice

import (
	"context"
	"fmt"
	"strings"
)

// ProfileForPrompt publishes the stable prefix parts. The return order is deliberate:
// consumers append styleguide, excerpts, then user-owned rules last.
func (s *Service) ProfileForPrompt(ctx context.Context, userID, voiceID string) (string, []string, string, bool, error) {
	return s.ProfileForPromptForTopic(ctx, userID, voiceID, "", nil)
}

func (s *Service) ProfileForPromptForTopic(ctx context.Context, userID, voiceID, topic string, tags []string) (string, []string, string, bool, error) {
	projection, err := s.PromptProfileForTopic(ctx, userID, voiceID, topic, tags)
	if err != nil {
		return "", nil, "", false, err
	}
	// Earned rules FIRST now: the free-text position ahead of them is gone with the section
	// that owned it (change 16), and what remains of ManualRules is the refine step's
	// "save as rule" text.
	rules := projection.ActiveRules
	if projection.ManualRules != "" {
		if rules != "" {
			rules += "\n"
		}
		rules += projection.ManualRules
	}
	return projection.Styleguide, projection.Excerpts, rules, projection.Empty, nil
}

type PromptProfile struct {
	Styleguide, ActiveRules, ManualRules string
	Excerpts                             []string
	Empty                                bool
	SourceLanguage, TargetLanguage       Language
	Portable                             bool
}

// PromptProfileForLanguage publishes a deterministic full or portable projection for one
// concrete target. Voice owns the field selection so no consumer can accidentally leak
// source-language excerpts or lexical rules across languages.
func (s *Service) PromptProfileForLanguage(ctx context.Context, userID, voiceID string, target Language) (PromptProfile, error) {
	return s.PromptProfileForTopicAndLanguage(ctx, userID, voiceID, target, "", nil)
}

func (s *Service) PromptProfileForTopicAndLanguage(ctx context.Context, userID, voiceID string, target Language, topic string, tags []string) (PromptProfile, error) {
	if !target.Valid() {
		return PromptProfile{}, ErrLanguageRequired
	}
	return s.promptProfileForTopic(ctx, userID, voiceID, target, topic, tags)
}

// PromptProfileForTopic projects exactly one voice. Every row it reads is keyed by that
// voice, so a well-trained sibling voice contributes nothing — an empty voice prompts as
// empty rather than borrowing. A deleted voice is refused: nothing may be written in it.
func (s *Service) PromptProfileForTopic(ctx context.Context, userID, voiceID, topic string, tags []string) (PromptProfile, error) {
	voice, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, err
	}
	return s.promptProfileForTopic(ctx, userID, voiceID, voice.SourceLanguage, topic, tags)
}

func (s *Service) promptProfileForTopic(ctx context.Context, userID, voiceID string, target Language, topic string, tags []string) (PromptProfile, error) {
	voice, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, err
	}
	// Retirement is evaluated only inside this explicit consumer request. No timer or
	// read-only profile page mutates lifecycle state.
	if err := s.retireStaleRules(ctx, userID, voiceID); err != nil {
		return PromptProfile{}, err
	}
	profile, err := s.store.GetProfile(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("get profile for prompt: %w", err)
	}
	if target != voice.SourceLanguage {
		style := renderPortableProfile(profile.Structured)
		return PromptProfile{
			Styleguide: style, Empty: style == "", SourceLanguage: voice.SourceLanguage,
			TargetLanguage: target, Portable: true,
		}, nil
	}
	samples, err := s.store.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("list excerpts: %w", err)
	}
	var sources []AuthoredSource
	if s.personalization != nil {
		sources, err = s.personalization.ListAuthoredSources(ctx, userID, voiceID)
	}
	if err != nil {
		return PromptProfile{}, fmt.Errorf("list authored excerpts: %w", err)
	}
	sources = authoredSourcesForLanguage(sources, voice.SourceLanguage)
	excerptLimit, excerptChars := s.config.FewShotMax, s.config.FewShotExcerptMaxChars
	if !s.personalizationReady {
		excerptLimit, excerptChars = ExcerptCount, ExcerptChars
	}
	excerpts := rankExcerpts(sources, topic, tags, excerptLimit)
	for _, sample := range samples {
		if len(excerpts) >= excerptLimit {
			break
		}
		candidate := excerptAroundTarget(sample.Body, s.config.FewShotExcerptTargetChars, excerptChars)
		if !s.personalizationReady {
			candidate = firstRunes(sample.Body, excerptChars)
		}
		if !containsString(excerpts, candidate) {
			excerpts = append(excerpts, candidate)
		}
	}
	var rules []ContrastRule
	if s.personalization != nil {
		rules, err = s.personalization.ListRules(ctx, userID, voiceID)
	}
	if err != nil {
		return PromptProfile{}, fmt.Errorf("list active rules: %w", err)
	}
	active := make([]string, 0)
	for _, rule := range rules {
		if rule.Status == RuleActive {
			active = append(active, rule.Statement)
		}
	}
	// ONE representation, injected ONCE. The analysis text reaches the model through the
	// structured profile's lexical description and nowhere else; the `[Legacy manual guidance]`
	// append that repeated the very same text under its own header is gone (change 16).
	style := renderStructuredProfileForLanguage(profile.Structured, voice.SourceLanguage)
	// An empty voice is now exactly "nothing to learn from and nothing published": no samples,
	// no authored sources, no structured version.
	empty := len(samples) == 0 && len(sources) == 0 && profile.Structured.Version == 0
	return PromptProfile{Styleguide: style, ActiveRules: strings.Join(active, "\n"), ManualRules: strings.TrimSpace(profile.Rules), Excerpts: excerpts, Empty: empty, SourceLanguage: voice.SourceLanguage, TargetLanguage: target}, nil
}

func (s *Service) retireStaleRules(ctx context.Context, userID, voiceID string) error {
	if s.config.RuleRetireAfter <= 0 || s.personalization == nil {
		return nil
	}
	now := s.now()
	if _, err := s.personalization.RetireStaleRulesAndPublish(ctx, userID, voiceID, now.Add(-s.config.RuleRetireAfter), now); err != nil {
		return fmt.Errorf("retire stale voice rules: %w", err)
	}
	return nil
}
