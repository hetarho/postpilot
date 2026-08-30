package voice

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) AppendRule(ctx context.Context, userID, voiceID, line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return err
	}
	s.profileMu.Lock()
	defer s.profileMu.Unlock()
	profile, err := s.store.GetProfile(ctx, userID, voiceID)
	if err != nil {
		return fmt.Errorf("get profile for rule: %w", err)
	}
	current := strings.TrimSpace(profile.Rules)
	for _, existing := range strings.Split(current, "\n") {
		if existing == line {
			return nil
		}
	}
	if current != "" {
		current += "\n"
	}
	if err := s.store.SetRules(ctx, userID, voiceID, current+line, s.now()); err != nil {
		return fmt.Errorf("append rule: %w", err)
	}
	return nil
}
