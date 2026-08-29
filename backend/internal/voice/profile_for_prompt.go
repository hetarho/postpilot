package voice

import (
	"context"
	"fmt"
	"strings"
)

// ProfileForPrompt publishes the stable prefix parts. The return order is deliberate:
// consumers append styleguide, excerpts, then user-owned rules last.
func (s *Service) ProfileForPrompt(ctx context.Context, userID string) (string, []string, string, bool, error) {
	return s.ProfileForPromptForTopic(ctx, userID, "", nil)
}

func (s *Service) ProfileForPromptForTopic(ctx context.Context, userID, topic string, tags []string) (string, []string, string, bool, error) {
	projection, err := s.PromptProfileForTopic(ctx, userID, topic, tags)
	if err != nil {
		return "", nil, "", false, err
	}
	rules := projection.ManualRules
	if projection.ActiveRules != "" {
		if rules != "" {
			rules += "\n"
		}
		rules += projection.ActiveRules
	}
	return projection.Styleguide, projection.Excerpts, rules, projection.Empty, nil
}

type PromptProfile struct {
	Styleguide, ActiveRules, ManualRules string
	Excerpts                             []string
	Empty                                bool
}

func (s *Service) PromptProfileForTopic(ctx context.Context, userID, topic string, tags []string) (PromptProfile, error) {
	// Retirement is evaluated only inside this explicit consumer request. No timer or
	// read-only profile page mutates lifecycle state.
	if err := s.retireStaleRules(ctx, userID); err != nil {
		return PromptProfile{}, err
	}
	profile, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("get profile for prompt: %w", err)
	}
	samples, err := s.store.ListSampleBodies(ctx, userID)
	if err != nil {
		return PromptProfile{}, fmt.Errorf("list excerpts: %w", err)
	}
	var sources []AuthoredSource
	if s.personalization != nil {
		sources, err = s.personalization.ListAuthoredSources(ctx, userID)
	}
	if err != nil {
		return PromptProfile{}, fmt.Errorf("list authored excerpts: %w", err)
	}
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
		rules, err = s.personalization.ListRules(ctx, userID)
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
	style := renderStructuredProfile(profile.Structured)
	if profile.Styleguide != "" {
		if !s.personalizationReady {
			style = profile.Styleguide
		} else {
			if style != "" {
				style += "\n\n"
			}
			style += "[Legacy manual guidance]\n" + profile.Styleguide
		}
	}
	empty := profile.Styleguide == "" && len(samples) == 0 && len(sources) == 0 && profile.Structured.Version == 0
	return PromptProfile{Styleguide: style, ActiveRules: strings.Join(active, "\n"), ManualRules: strings.TrimSpace(profile.Rules), Excerpts: excerpts, Empty: empty}, nil
}

func (s *Service) retireStaleRules(ctx context.Context, userID string) error {
	if s.config.RuleRetireAfter <= 0 || s.personalization == nil {
		return nil
	}
	now := s.now()
	if _, err := s.personalization.RetireStaleRulesAndPublish(ctx, userID, now.Add(-s.config.RuleRetireAfter), now); err != nil {
		return fmt.Errorf("retire stale voice rules: %w", err)
	}
	return nil
}
