package main

import (
	"context"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// testVoiceName is the voice a test account creates for itself.
const testVoiceName = "기본 말투"

// createTestVoice gives a test account one voice by name, the way an owner makes their first:
// no bootstrap creates one (VOICE-4).
func createTestVoice(ctx context.Context, handle *db.DB, userID string) error {
	_, err := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil).CreateVoice(ctx, userID, testVoiceName)
	return err
}

// firstTestVoice is the account's first active voice in directory order.
func firstTestVoice(ctx context.Context, svc *voice.Service, userID string) (voice.Voice, error) {
	voices, err := svc.ListVoices(ctx, userID)
	if err != nil {
		return voice.Voice{}, err
	}
	for _, found := range voices {
		if !found.Deleted() {
			return found, nil
		}
	}
	return voice.Voice{}, voice.ErrVoiceNotFound
}
