package main

import (
	"context"
	"github.com/postpilot/backend/internal/voice"
)

type clipUploadVoice struct{ service *voice.Service }

func (a clipUploadVoice) DefaultClipVoice(ctx context.Context, user, language string) (string, error) {
	voices, err := a.service.ListVoices(ctx, user)
	if err != nil {
		return "", err
	}
	for _, found := range voices {
		if !found.IsDefault || found.Deleted() || !found.Made {
			continue
		}
		profile, err := a.service.PromptProfileForTopic(ctx, user, found.ID, "", voice.Language(language), "")
		if err != nil {
			return "", err
		}
		// Style alone crosses this boundary; no sample, excerpt or foreign experience.
		return profile.Text, nil
	}
	return "", nil
}
