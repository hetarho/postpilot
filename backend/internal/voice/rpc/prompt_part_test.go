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

// ARCH-3: every generated AI field but UNSPECIFIED is one the domain has.
func TestEveryAIFieldMaps(t *testing.T) {
	seen := map[postpilotv1.VoiceAiField]bool{}
	for _, field := range []voice.AIField{voice.AIImpression, voice.AITics, voice.AISignaturePhrases} {
		mapped := toProtoAIField(field)
		if mapped == postpilotv1.VoiceAiField_VOICE_AI_FIELD_UNSPECIFIED || seen[mapped] {
			t.Fatalf("%q maps to %v", field, mapped)
		}
		seen[mapped] = true
	}
	if len(seen) != len(postpilotv1.VoiceAiField_name)-1 {
		t.Fatalf("the domain maps %d of %d generated fields", len(seen), len(postpilotv1.VoiceAiField_name)-1)
	}
}
