package clip

import (
	"testing"
)

func TestSpeechFingerprintMatchesBrowserUTF8Framing(t *testing.T) {
	speech := []SpeechPlacement{{SegmentID: "spoken-1", StartMS: 1000, EndMS: 2000, VolumePermille: 700, Speech: SpeechRef{AssetID: "asset", VoiceID: "voice", BindingDigest: "binding", InputHash: "input", SettingsHash: "settings", AudioHash: "audio", ProfileID: "profile", ProfileRevision: 1, Samples: 44100, SampleRate: 44100, Channels: 2, Timing: []SpeechTiming{{Text: "한글", StartMS: 0, EndMS: 1000}}}}}
	if got := SpeechFingerprint(speech); got != "79a6051cecf7e220ba310bb6fe4705314f2d9cbf8de6b071118abaee9e40f23f" {
		t.Fatal(got)
	}
	original := SpeechFingerprint(speech)
	speech[0].VolumePermille++
	if SpeechFingerprint(speech) == original {
		t.Fatal("gain omitted")
	}
	if SpeechFingerprint(nil) != "" {
		t.Fatal("ordinary export fingerprint")
	}
}
