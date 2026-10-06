package store

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestBrowserCompositionCodecKeepsOldArraysAndOwnEnvelope(t *testing.T) {
	speech := []clip.SpeechPlacement{{SegmentID: "speech"}}
	old, err := encodeResultSpeech(speech)
	if err != nil {
		t.Fatal(err)
	}
	got, contract, err := decodeBrowserComposition(old)
	if err != nil || contract != nil || !reflect.DeepEqual(got, speech) {
		t.Fatal(got, contract, err)
	}
	for _, raw := range []string{"", "[]"} {
		got, contract, err = decodeBrowserComposition(raw)
		if err != nil || len(got) != 0 || contract != nil {
			t.Fatal(got, contract, err)
		}
	}
	binding := clip.BrowserCompositionContract{Version: clip.BrowserCompositionVersion, SnapshotFingerprint: strings.Repeat("a", 64), Components: clip.BrowserComponentVersion, Fonts: clip.BrowserFontVersion, Assets: clip.BrowserAssetVersion}
	raw, err := encodeBrowserComposition(clip.BrowserRender{Composition: &binding, Speech: speech})
	if err != nil {
		t.Fatal(err)
	}
	got, contract, err = decodeBrowserComposition(raw)
	if err != nil || !reflect.DeepEqual(got, speech) || contract == nil || *contract != binding {
		t.Fatal(got, contract, err)
	}
	if _, err := decodeResultSpeech(raw); err == nil {
		t.Fatal("browser envelope broadened result aggregate codec")
	}
	for _, invalid := range []string{
		strings.Replace(raw, `"version":1`, `"version":2`, 1),
		strings.Replace(raw, clip.BrowserCompositionVersion, "future", 1),
		strings.TrimSuffix(raw, "}") + `,"unknown":true}`,
		raw + `{}`,
	} {
		if _, _, err := decodeBrowserComposition(invalid); err == nil {
			t.Fatal("accepted invalid browser envelope", invalid)
		}
	}
}
