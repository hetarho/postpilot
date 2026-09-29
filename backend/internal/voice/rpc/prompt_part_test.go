package rpc

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice"
)

// ARCH-3: every generated prompt part but UNSPECIFIED is one the domain has, and each domain
// part maps to its own value.
func TestEveryPromptPartMapsBothWays(t *testing.T) {
	seen := map[postpilotv1.VoicePromptPart]voice.PromptPart{}
	for _, part := range voice.Parts() {
		mapped := toProtoPart(part)
		if mapped == postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_UNSPECIFIED {
			t.Fatalf("%q maps to UNSPECIFIED", part)
		}
		if other, dup := seen[mapped]; dup {
			t.Fatalf("%q and %q share %v", part, other, mapped)
		}
		seen[mapped] = part
	}
	for value := range postpilotv1.VoicePromptPart_name {
		part := postpilotv1.VoicePromptPart(value)
		if part == postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_UNSPECIFIED {
			continue
		}
		if _, ok := seen[part]; !ok {
			t.Fatalf("generated %v has no domain part", part)
		}
	}
}
