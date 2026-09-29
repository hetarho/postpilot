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

// ARCH-3: every counted item and facet unit maps to its own generated value, and every generated
// value but UNSPECIFIED is one the domain has.
func TestEveryFingerprintItemAndUnitMaps(t *testing.T) {
	items := map[postpilotv1.FingerprintItem]bool{}
	for _, item := range voice.Items() {
		mapped := toProtoItem(item)
		if mapped == postpilotv1.FingerprintItem_FINGERPRINT_ITEM_UNSPECIFIED || items[mapped] {
			t.Fatalf("%q maps to %v", item, mapped)
		}
		items[mapped] = true
	}
	if len(items) != len(postpilotv1.FingerprintItem_name)-1 {
		t.Fatalf("the domain maps %d of %d generated items", len(items), len(postpilotv1.FingerprintItem_name)-1)
	}
	units := map[postpilotv1.FingerprintFacetUnit]bool{}
	for _, unit := range voice.FacetUnits() {
		mapped := toProtoFacetUnit(unit)
		if mapped == postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_UNSPECIFIED || units[mapped] {
			t.Fatalf("%q maps to %v", unit, mapped)
		}
		units[mapped] = true
	}
	if len(units) != len(postpilotv1.FingerprintFacetUnit_name)-1 {
		t.Fatalf("the domain maps %d of %d generated units", len(units), len(postpilotv1.FingerprintFacetUnit_name)-1)
	}
}
