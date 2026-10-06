package rpc

import (
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice"
	"testing"
)

func TestEveryOriginMapsAndLegacyDefaultsToPersonal(t *testing.T) {
	seen := map[postpilotv1.VoiceOrigin]bool{}
	for _, origin := range []voice.Origin{voice.OriginPersonal, voice.OriginSynthetic} {
		mapped := toProtoOrigin(origin)
		if mapped == postpilotv1.VoiceOrigin_VOICE_ORIGIN_UNSPECIFIED || seen[mapped] {
			t.Fatalf("origin=%q mapped=%v", origin, mapped)
		}
		seen[mapped] = true
	}
	for value := range postpilotv1.VoiceOrigin_name {
		if value != 0 && !seen[postpilotv1.VoiceOrigin(value)] {
			t.Fatalf("unmapped generated origin=%d", value)
		}
	}
	if toProtoOrigin("") != postpilotv1.VoiceOrigin_VOICE_ORIGIN_PERSONAL {
		t.Fatal("legacy origin changed")
	}
}
