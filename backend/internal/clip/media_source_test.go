package clip_test

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestMediaOriginalRebindChecksRecordedAudioAndCadence(t *testing.T) {
	expected := clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080, HasAudio: true, AudioRate: 48000, AudioChannels: 2, FrameRateNumerator: 60000, FrameRateDenominator: 1001, CadenceVerified: true}
	if !clip.SameMediaOriginal(expected, expected) {
		t.Fatal("original refused")
	}
	for _, mutate := range []func(*clip.MediaInfo){
		func(i *clip.MediaInfo) { i.DurationMS++ }, func(i *clip.MediaInfo) { i.Width++ }, func(i *clip.MediaInfo) { i.HasAudio = false }, func(i *clip.MediaInfo) { i.AudioRate = 44100 }, func(i *clip.MediaInfo) { i.AudioChannels = 1 }, func(i *clip.MediaInfo) { i.CadenceVerified = false }, func(i *clip.MediaInfo) { i.FrameRateNumerator = 30000 },
	} {
		actual := expected
		mutate(&actual)
		if clip.SameMediaOriginal(expected, actual) {
			t.Fatal("changed original accepted")
		}
	}
	actual := expected
	actual.FrameRateNumerator *= 2
	actual.FrameRateDenominator *= 2
	if !clip.SameMediaOriginal(expected, actual) {
		t.Fatal("equivalent rational frame rate refused")
	}
	legacy := expected
	legacy.AudioRate = 0
	legacy.AudioChannels = 0
	legacy.CadenceVerified = false
	legacy.FrameRateNumerator = 0
	legacy.FrameRateDenominator = 0
	if !clip.SameMediaOriginal(legacy, actual) {
		t.Fatal("missing legacy measurements block compatible original")
	}
}

func TestFingerprintIgnoresCaptureTime(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	first, second := now.Add(-time.Hour), now.Add(-2*time.Hour)
	metadata := clip.SourceMetadata{Filename: "camera.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: 1000, Width: 1920, Height: 1080, Fingerprint: "same-original", CapturedAt: &first}
	frozen := clip.MediaTaskSource{ID: "retained", SourceMetadata: metadata}
	lease := clip.SourceLease{ID: "reselected", SourceMetadata: metadata, ActualBytes: 100, State: "ready", ExpiresAt: now.Add(time.Hour)}
	lease.CapturedAt = &second
	batch := clip.SourceBatch{Sources: []clip.SourceLease{lease}}
	if _, err := clip.ResolveMediaSource(clip.MediaRender, frozen, batch, now); err != nil {
		t.Fatal("capture changed fingerprint rebind", err)
	}
	batch.Sources[0].DurationMS++
	if _, err := clip.ResolveMediaSource(clip.MediaRender, frozen, batch, now); err == nil {
		t.Fatal("changed original metadata accepted")
	}
}
