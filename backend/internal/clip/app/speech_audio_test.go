package app

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"testing"
	"time"
)

func TestExpiredSpeechPlaybackIsRefusedBeforePrivateStorageIO(t *testing.T) {
	s := &SpeechService{playback: map[string]speechPlayback{"expired": {owner: "alice", expires: time.Now().Add(-time.Second)}}}
	if _, e := s.ReadSpeechPlayback(t.Context(), "alice", "expired"); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := s.ReadSpeechPlayback(t.Context(), "alice", "unknown"); !errors.Is(e, clip.ErrNotFound) {
		t.Fatal(e)
	}
}
