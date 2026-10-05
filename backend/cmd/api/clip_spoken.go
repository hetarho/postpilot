package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice/spoken"
)

// Root adapter: clip owns this consumer port; the spoken context owns the
// handle and immutable profile. No clip query reads spoken tables.
type clipSpokenVoices struct{ library *spoken.Service }

func (a clipSpokenVoices) ResolveClipVoice(ctx context.Context, owner, id string) (clip.SpokenVoiceBinding, error) {
	v, err := a.library.GetVoice(ctx, owner, id)
	if err != nil {
		return clip.SpokenVoiceBinding{}, err
	}
	if v.RemovedAt != nil {
		return clip.SpokenVoiceBinding{}, clip.ErrNotFound
	}
	b, err := json.Marshal(struct {
		ID      string
		Handle  llm.VoiceHandle
		Profile spoken.Profile
	}{v.ID, v.Handle, v.Profile})
	if err != nil {
		return clip.SpokenVoiceBinding{}, err
	}
	d := sha256.Sum256(b)
	return clip.SpokenVoiceBinding{ID: v.ID, Digest: hex.EncodeToString(d[:])}, nil
}
