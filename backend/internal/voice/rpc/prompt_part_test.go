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

// ARCH-3: every counted item, facet unit and verdict maps to its own generated value, and every generated
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
	verdicts := map[postpilotv1.FingerprintVerdict]bool{}
	for _, verdict := range voice.Verdicts() {
		mapped := toProtoVerdict(verdict)
		if mapped == postpilotv1.FingerprintVerdict_FINGERPRINT_VERDICT_UNSPECIFIED || verdicts[mapped] {
			t.Fatalf("%q maps to %v", verdict, mapped)
		}
		verdicts[mapped] = true
	}
	if len(verdicts) != len(postpilotv1.FingerprintVerdict_name)-1 {
		t.Fatalf("the domain maps %d of %d generated verdicts", len(verdicts), len(postpilotv1.FingerprintVerdict_name)-1)
	}
	if toProtoVerdict(voice.VerdictNone) != postpilotv1.FingerprintVerdict_FINGERPRINT_VERDICT_UNSPECIFIED {
		t.Fatal("no verdict must stay UNSPECIFIED")
	}
}

// ARCH-3, VOICE-77: every generated habit kind but UNSPECIFIED is one the domain has.
func TestEveryHabitKindMapsBothWays(t *testing.T) {
	kinds := []voice.HabitKind{voice.HabitEnding, voice.HabitAdverb, voice.HabitOpening, voice.HabitClosing, voice.HabitSignature}
	seen := map[postpilotv1.VoiceHabitKind]bool{}
	for _, kind := range kinds {
		mapped := toProtoHabitKind(kind)
		if mapped == postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_UNSPECIFIED || seen[mapped] {
			t.Fatalf("%q maps to %v", kind, mapped)
		}
		seen[mapped] = true
	}
	if len(seen) != len(postpilotv1.VoiceHabitKind_name)-1 {
		t.Fatalf("the domain maps %d of %d generated kinds", len(seen), len(postpilotv1.VoiceHabitKind_name)-1)
	}
}

// VOICE-77: a habit in a TEXT block or in one LIST item is located by the matching field kind,
// with the offsets and the voice's own numbers carried over.
func TestHabitsLocateTheirField(t *testing.T) {
	got := toProtoHabits([]voice.HabitPhrase{
		{BlockIndex: 2, ItemIndex: -1, Start: 3, End: 9, Quote: "진짜", Kind: voice.HabitAdverb, Term: "진짜", VoicePerHundred: 18, TextCount: 2, Example: "진짜 좋아요."},
		{BlockIndex: 4, ItemIndex: 1, Start: 0, End: 2, Quote: "면", Kind: voice.HabitSignature, Term: "면", ExampleFictional: true},
	})
	if len(got) != 2 {
		t.Fatalf("habits = %+v", got)
	}
	block, item := got[0].GetField(), got[1].GetField()
	if block.GetKind() != postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_CONTENT || block.BlockIndex == nil || block.GetBlockIndex() != 2 || block.ItemIndex != nil {
		t.Fatalf("block field = %+v", block)
	}
	if item.GetKind() != postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_ITEM || item.GetBlockIndex() != 4 || item.ItemIndex == nil || item.GetItemIndex() != 1 {
		t.Fatalf("item field = %+v", item)
	}
	first := got[0]
	if first.GetStart() != 3 || first.GetEnd() != 9 || first.GetQuote() != "진짜" || first.GetKind() != postpilotv1.VoiceHabitKind_VOICE_HABIT_KIND_ADVERB ||
		first.GetVoicePerHundred() != 18 || first.GetTextCount() != 2 || first.GetExample() != "진짜 좋아요." || got[1].GetExampleFictional() != true {
		t.Fatalf("habit = %+v", first)
	}
}
