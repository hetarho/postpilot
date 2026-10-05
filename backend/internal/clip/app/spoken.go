package app

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
)

func (s *GenerationService) bindNarrationVoice(ctx context.Context, owner string, old clip.Project, next *clip.EditPlan) error {
	n := next.Narration
	if n == nil || n.VoiceID == "" {
		return nil
	}
	previous, err := clip.DecodeEditPlan(old.EditPlan)
	if err != nil {
		return err
	}
	// A removed voice keeps its immutable existing speech. Only new selection
	// resolves the live library; generation resolves it again before paid work.
	if previous.Narration != nil && n.VoiceID == previous.Narration.VoiceID && n.BindingDigest != "" && n.BindingDigest == previous.Narration.BindingDigest {
		return nil
	}
	binding, err := s.voices.ResolveClipVoice(ctx, owner, n.VoiceID)
	if err != nil {
		return err
	}
	if binding.ID != n.VoiceID || binding.Digest == "" {
		return clip.ErrInvalid
	}
	n.BindingDigest = binding.Digest
	return clip.ValidateNarration(*next)
}
