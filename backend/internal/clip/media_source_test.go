package clip_test

import (
	"github.com/postpilot/backend/internal/clip"
	"testing"
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
