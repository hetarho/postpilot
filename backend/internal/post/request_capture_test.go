package post

import "testing"

func TestRequestCaptureSourceIdentityIgnoresOwnResultsAndFencesOwnerChanges(t *testing.T) {
	p := Post{Title: "Owner title", Memo: "Owner material", TargetLanguage: LanguageEnglish, TagCount: 3, Images: []Image{{ID: "photo-1", Filename: "same.jpg"}}, TemplateAnswers: []TemplateAnswer{{Label: "Fact", Text: "Owner answer", Enabled: true}}}
	identity := RequestCaptureSourceFingerprint(p)
	p.InputRevision++
	p.ContentRevision++
	p.Content = &PostContent{Title: "Machine title"}
	p.Observations = []Observation{{File: "same.jpg"}}
	p.Storyline = &Storyline{Paragraphs: []StorylineParagraph{{Text: "Machine plan"}}}
	p.Images[0].Rotation = 90
	if RequestCaptureSourceFingerprint(p) != identity {
		t.Fatal("own execution invalidated source fingerprint")
	}
	p.Images[0].RotationByOwner = true
	if RequestCaptureSourceFingerprint(p) == identity {
		t.Fatal("owner rotation reused old source witness")
	}
	p.Images[0].RotationByOwner = false
	p.Images[0].ID = "photo-replacement"
	if RequestCaptureSourceFingerprint(p) == identity {
		t.Fatal("same filename retargeted withdrawn source")
	}
	p.Images[0].ID = "photo-1"
	p.TemplateAnswers[0].Text = "New answer"
	if RequestCaptureSourceFingerprint(p) == identity {
		t.Fatal("owner answer edit reused old request")
	}
}
