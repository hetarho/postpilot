package generation

import (
	"strings"
	"testing"
)

// noVoiceWriteInput is a 말투 없음 post with every section that names a voice elsewhere: a
// template, both guideline groups and a Korean target length.
func noVoiceWriteInput() WritePromptInput {
	target := 1200
	return WritePromptInput{
		Language: LanguageKorean, Profile: Profile{NoVoice: true, TargetLanguage: LanguageKorean},
		Observations: goldenObservations(), Memo: "MEMO 본문", Title: "가제 TITLE", Photos: []string{"IMG_1.jpg", "IMG_2.jpg"},
		TargetLength: &target, TagCount: 4, Template: testBrief(),
		DefaultGuidelines: productDefaults(LanguageKorean), Guidelines: []string{"가격은 쓰지 않기"},
	}
}

// voiceBytes are what only a voice puts into a prompt (GEN-74).
var voiceBytes = []string{"말투 프로필", "[스타일가이드]", "[글 예시 발췌]", "[종결어미 제약]", "휴대 가능한 말투 프로필", "voice profile"}

// GEN-74, TMPL-12, GUIDE-15: a 말투 없음 write carries no voice section and no voice clause —
// the output-language sentence, the template and guideline precedence sentences take their
// no-voice forms — and the Korean length stands on its own [길이] line.
func TestANoVoiceWritePromptCarriesNoVoiceBytes(t *testing.T) {
	system, user := BuildWritePromptForLanguage(noVoiceWriteInput())
	wantSystem, wantUser := loadGolden(t, "write_prompt_no_voice.golden")
	if system != wantSystem || user != wantUser {
		t.Fatalf("the 말투 없음 write prompt drifted:\n--- got ---\n%s\n@@USER@@\n%s", system, user)
	}
	for _, forbidden := range voiceBytes {
		if strings.Contains(system, forbidden) {
			t.Errorf("a 말투 없음 write prompt carries %q", forbidden)
		}
	}
	for _, required := range []string{templatePrecedenceNoVoice, guidelinePrecedenceNoVoice, "\n\n[길이]\n목표 길이: 약 1200자."} {
		if !strings.Contains(system, required) {
			t.Errorf("a 말투 없음 write prompt lacks %q", required)
		}
	}
}

// GEN-40, GEN-74: the same for a revision, whose literal line points at the current post's
// style instead of a profile.
func TestANoVoiceRevisePromptCarriesNoVoiceBytes(t *testing.T) {
	target := 1200
	system, user := BuildRevisePromptForLanguage(LanguageKorean, Profile{NoVoice: true, TargetLanguage: LanguageKorean}, goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", &target, 4, testBrief(),
		FrozenGuidelines{Defaults: productDefaults(LanguageKorean), Owner: []string{"가격은 쓰지 않기"}})
	wantSystem, wantUser := loadGolden(t, "revise_prompt_no_voice.golden")
	if system != wantSystem || user != wantUser {
		t.Fatalf("the 말투 없음 revise prompt drifted:\n--- got ---\n%s\n@@USER@@\n%s", system, user)
	}
	for _, forbidden := range voiceBytes {
		if strings.Contains(system, forbidden) {
			t.Errorf("a 말투 없음 revise prompt carries %q", forbidden)
		}
	}
	if !strings.Contains(system, koreanReviseLiteralNoVoice) || strings.Contains(system, koreanReviseLiteral) {
		t.Error("the 말투 없음 revise prompt kept the voice literal")
	}
}

// An English 말투 없음 target names no voice in its output-language sentence either.
func TestANoVoiceEnglishWriteNamesNoVoice(t *testing.T) {
	input := noVoiceWriteInput()
	input.Language, input.Profile.TargetLanguage = LanguageEnglish, LanguageEnglish
	input.DefaultGuidelines = productDefaults(LanguageEnglish)
	system, _ := BuildWritePromptForLanguage(input)
	for _, forbidden := range voiceBytes {
		if strings.Contains(system, forbidden) {
			t.Errorf("an English 말투 없음 write prompt carries %q", forbidden)
		}
	}
	if !strings.Contains(system, "\n\n[Length]\nTarget approximately 1200 Unicode characters.") {
		t.Error("an English 말투 없음 write lost its length line")
	}
}
