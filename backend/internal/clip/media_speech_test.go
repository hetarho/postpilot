package clip

import (
	"strings"
	"testing"
)

func TestMediaSpeechRequiresExactBoundedManifestAndCFRDuration(t *testing.T) {
	hash := strings.Repeat("a", 64)
	ref := SpeechRef{AssetID: "asset", VoiceID: "voice", BindingDigest: hash, InputHash: SpokenInputHash("speech"), SettingsHash: hash, AudioHash: hash, ProfileID: "profile", ProfileRevision: 1, Samples: 44100, SampleRate: 44100, Channels: 2}
	p := EditPlan{DurationMS: 15010, Cuts: []Cut{{StartMS: 0, EndMS: 15010}}, Narration: &NarrationPlan{Enabled: true, VoiceID: "voice", BindingDigest: hash, VolumePermille: 1000, Segments: []SpokenSegment{{ID: "spoken-1", Text: "speech", TextRevision: 1, InputHash: ref.InputHash, EndMS: 1000, Speech: &ref}}}}
	task := MediaTask{Speech: []MediaTaskSpeech{{AssetID: "asset", AudioHash: hash, Bytes: 100}}}
	if err := ValidateMediaSpeech(task, p); err != nil {
		t.Fatal(err)
	}
	for _, refs := range [][]MediaTaskSpeech{nil, {{AssetID: "asset", AudioHash: hash, Bytes: SpeechMaxAssetBytes + 1}}, {{AssetID: "asset", AudioHash: strings.Repeat("b", 64), Bytes: 100}}, append(task.Speech, task.Speech...)} {
		task.Speech = refs
		if ValidateMediaSpeech(task, p) == nil {
			t.Fatal("invalid manifest admitted", refs)
		}
	}
	p.Narration.Segments[0].StartMS = 14010
	p.Narration.Segments[0].EndMS = 15010
	if NarrationReadiness(p) != nil {
		t.Fatal("valid draft rejected")
	}
	if OutputNarrationReadiness(p, 30, 48000) == nil {
		t.Fatal("last requested samples would be clipped by CFR rounding")
	}
}
