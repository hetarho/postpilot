package post

import (
	"errors"
	"testing"
)

func TestTestAssignmentsHashUsesAssignmentsAndOptionsInCanonicalOrder(t *testing.T) {
	p := Post{VoiceID: "voice", TemplateID: "template", TargetLanguage: LanguageKorean, TagCount: TagCountRange.Default, QualityRules: []string{"length", "links"}}
	first := TestAssignmentsHash(p)
	p.QualityRules = []string{"links", "length"}
	if TestAssignmentsHash(p) != first {
		t.Fatal("quality order changed assignment identity")
	}
	p.Title = "Material lives behind its revision fence"
	if TestAssignmentsHash(p) != first {
		t.Fatal("assignment identity includes material")
	}
	p.VoiceID = "other voice"
	if TestAssignmentsHash(p) == first {
		t.Fatal("voice change preserved frozen assignments")
	}
}

func TestWinnerStorylineRequiresCurrentAttachmentsAndMachineProvenance(t *testing.T) {
	photos := []Image{{Filename: "photo.jpg"}}
	valid := &Storyline{MadeWith: []string{"photo.jpg"}, Paragraphs: []StorylineParagraph{{Text: "Plan", Files: []string{"photo.jpg"}}}}
	if err := ValidateTestStoryline(valid, photos, nil); err != nil {
		t.Fatal(err)
	}
	valid.EditedByHand = true
	if err := ValidateTestStoryline(valid, photos, nil); !errors.Is(err, ErrStorylineInvalid) {
		t.Fatalf("hand-edit claim = %v", err)
	}
	valid.EditedByHand = false
	if err := ValidateTestStoryline(valid, nil, nil); err == nil {
		t.Fatal("detached photo was accepted")
	}
	valid.Paragraphs = append(valid.Paragraphs, valid.Paragraphs[0])
	if err := ValidateTestStoryline(valid, photos, nil); !errors.Is(err, ErrStorylineInvalid) {
		t.Fatalf("duplicate storyline attachment = %v", err)
	}
}

func TestTestResultServiceRequiresPublicationStore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("missing publication store silently allowed")
		}
	}()
	NewTestResultService(nil)
}
