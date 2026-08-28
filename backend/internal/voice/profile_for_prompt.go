package voice

import (
	"context"
	"fmt"
)

// ProfileForPrompt publishes the stable prefix parts. The return order is deliberate:
// consumers append styleguide, excerpts, then user-owned rules last.
func (s *Service) ProfileForPrompt(ctx context.Context, userID string) (string, []string, string, bool, error) {
	profile, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		return "", nil, "", false, fmt.Errorf("get profile for prompt: %w", err)
	}
	samples, err := s.store.ListSampleBodies(ctx, userID)
	if err != nil {
		return "", nil, "", false, fmt.Errorf("list excerpts: %w", err)
	}
	limit := min(ExcerptCount, len(samples))
	excerpts := make([]string, 0, limit)
	for _, sample := range samples[:limit] {
		excerpts = append(excerpts, firstRunes(sample.Body, ExcerptChars))
	}
	empty := profile.Styleguide == "" && len(samples) == 0
	return profile.Styleguide, excerpts, profile.Rules, empty, nil
}
