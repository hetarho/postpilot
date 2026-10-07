package template

import (
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

func TestLegacyTemplateCorrectionCompositionUsesActualTurnsAndAvailability(t *testing.T) {
	in := requestInput{Language: LanguageEnglish, Text: "change layout", Draft: Draft{Name: "draft", Body: "body"}, Sample: &Sample{Title: "sample", Text: "reference"}}
	turns := []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(requestMessage(in))}}, {Role: llm.RoleAssistant, Parts: []llm.Part{llm.TextPart("previous candidate")}}, {Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(correctionMessage(in.Language, ErrNameRequired))}}}
	c := templateRequestComposition(in, "actual formatted contract", turns)
	got, err := llm.PreparedRequestInspection(llm.Request{System: "actual formatted contract", Messages: turns, Composition: c, JSONSchema: RequestAnswerSchema()})
	if err != nil {
		t.Fatal(err)
	}
	var user strings.Builder
	for _, f := range got.Fragments {
		if f.Role == llm.InspectionRoleUser && f.ID != "validation-feedback" {
			user.WriteString(f.Text)
		}
	}
	if user.String() != turns[0].Parts[0].Text {
		t.Fatal("inventory omitted actual original envelope bytes")
	}
	n := len(got.Fragments)
	if got.Mode != "correction" || !strings.Contains(got.Activation, "admitted-only") || got.Fragments[n-2].Role != llm.InspectionRoleAssistant || got.Fragments[n-2].Text != turns[1].Parts[0].Text || got.Fragments[n-1].Authorship != llm.FragmentAuthorshipCode || got.Fragments[n-1].Role != llm.InspectionRoleUser || got.Fragments[n-1].Text != turns[2].Parts[0].Text {
		t.Fatalf("correction contract did not describe actual turns: %+v", got)
	}
	if got.Fragments[n-3].MaterialRole != "structural-reference-not-event-evidence" || got.IssuedAt != nil || len(RequestCompositions()) != 2 {
		t.Fatal("reference or availability projection wrong")
	}
}
