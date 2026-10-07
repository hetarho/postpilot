package generation

import (
	"context"
	"errors"
)

type profileSourcePayload struct {
	SampleID        string `json:"sample_id"`
	ContentRevision int64  `json:"content_revision"`
}
type profilePayload struct {
	NoVoice  bool                   `json:"no_voice,omitempty"`
	Text     string                 `json:"text,omitempty"`
	Excerpts []string               `json:"excerpts,omitempty"`
	Portable bool                   `json:"portable,omitempty"`
	Sources  []profileSourcePayload `json:"sources,omitempty"`
}

func encodeProfile(profile *Profile) *profilePayload {
	if profile == nil {
		return nil
	}
	out := &profilePayload{NoVoice: profile.NoVoice, Text: profile.Text, Excerpts: cloneTexts(profile.Excerpts), Portable: profile.Portable}
	for _, source := range profile.Sources {
		out.Sources = append(out.Sources, profileSourcePayload{SampleID: source.SampleID, ContentRevision: source.ContentRevision})
	}
	return out
}
func decodeProfile(profile *profilePayload) *Profile {
	if profile == nil {
		return nil
	}
	out := &Profile{NoVoice: profile.NoVoice, Text: profile.Text, Excerpts: cloneTexts(profile.Excerpts), Portable: profile.Portable}
	for _, source := range profile.Sources {
		out.Sources = append(out.Sources, ProfileSource{SampleID: source.SampleID, ContentRevision: source.ContentRevision})
	}
	return out
}
func (s *Service) validateFrozenProfile(ctx context.Context, userID, voiceID string, profile *Profile) error {
	if profile == nil || profile.NoVoice {
		return nil
	}
	if checker, ok := s.profiles.(FrozenProfileSources); ok {
		return checker.ValidateProfileSources(ctx, userID, voiceID, profile.Sources)
	}
	if len(profile.Sources) > 0 {
		return errors.New("frozen voice source validation is unavailable")
	}
	return nil
}
