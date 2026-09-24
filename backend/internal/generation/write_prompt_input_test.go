package generation

import (
	"reflect"
	"testing"
)

// Every member of the write prompt's input changes the prompt: a member added without a fixture
// here, or one the builder never reads, fails — so what reaches the prompt stays one type.
func TestEveryWritePromptInputMemberReachesThePrompt(t *testing.T) {
	base := WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Photos: []string{"IMG_1.jpg"}, TagCount: 4}
	target := 1500
	fixtures := map[string]func(*WritePromptInput){
		"Language":     func(in *WritePromptInput) { in.Language = LanguageEnglish },
		"Profile":      func(in *WritePromptInput) { in.Profile = Profile{Styleguide: "OTHER"} },
		"Observations": func(in *WritePromptInput) { in.Observations = goldenObservations() },
		"Memo":         func(in *WritePromptInput) { in.Memo = "memo" },
		"Title":        func(in *WritePromptInput) { in.Title = "title" },
		"Photos":       func(in *WritePromptInput) { in.Photos = []string{"IMG_1.jpg", "IMG_2.jpg"} },
		"Videos":       func(in *WritePromptInput) { in.Videos = []string{"a.mp4"} },
		"TargetLength": func(in *WritePromptInput) { in.TargetLength = &target },
		"TagCount":     func(in *WritePromptInput) { in.TagCount = 7 },
		"Template":     func(in *WritePromptInput) { in.Template = testBrief() },
		"Guidelines":   func(in *WritePromptInput) { in.Guidelines = testGuidelines() },
		"Memories":     func(in *WritePromptInput) { in.Memories = testMemories() },
		"QualityRules": func(in *WritePromptInput) { in.QualityRules = testQualityRules() },
		"FieldPhrases": func(in *WritePromptInput) { in.FieldPhrases = []string{"성수 카페"} },
	}
	baseSystem, baseUser := BuildWritePromptForLanguage(base)
	members := reflect.TypeOf(WritePromptInput{})
	for i := 0; i < members.NumField(); i++ {
		name := members.Field(i).Name
		fixture, ok := fixtures[name]
		if !ok {
			t.Errorf("%s has no fixture: give it a fixture and make the builder read it", name)
			continue
		}
		changed := base
		fixture(&changed)
		system, user := BuildWritePromptForLanguage(changed)
		if system+"\x00"+user == baseSystem+"\x00"+baseUser {
			t.Errorf("%s changed nothing in the prompt: the builder never reads it", name)
		}
	}
	if len(fixtures) != members.NumField() {
		t.Errorf("%d fixtures for %d members: a fixture names a member that no longer exists", len(fixtures), members.NumField())
	}
}
