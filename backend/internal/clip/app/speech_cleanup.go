package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func (s *SpeechService) Cleanup(ctx context.Context, before time.Time) error {
	intents, err := s.Store.PendingSpeechCleanup(ctx, before)
	if err != nil {
		return err
	}
	for _, intent := range intents {
		if intent.ID == "" || intent.ObjectKey != clip.SpeechAudioPrefix+intent.ID+".mp3" {
			return clip.ErrInvalid
		}
		retained, err := s.Store.SpeechAssetRetained(ctx, intent.ID)
		if err != nil {
			return err
		}
		if retained {
			// Conditional removal cannot lose a concurrent cascade's fresh cleanup intent.
			if err := s.Store.DiscardRetainedSpeechCleanup(ctx, intent.ID); err != nil {
				return err
			}
			continue
		}
		if err := s.Objects.DeleteClipSpeechAudio(ctx, intent.ObjectKey); err != nil {
			return err
		}
		if err := s.Store.CompleteSpeechCleanup(ctx, intent.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SpeechService) RunCleanup(ctx context.Context, interval, minAge time.Duration) {
	if interval <= 0 {
		panic("clip speech cleanup interval required")
	}
	minAge = max(minAge, SpeechCleanupMinAge)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sweep, cancel := context.WithTimeout(ctx, time.Minute)
			if err := s.Cleanup(sweep, time.Now().Add(-minAge)); err != nil && ctx.Err() == nil {
				slog.Warn("clip speech cleanup pending")
			}
			cancel()
		}
	}
}
