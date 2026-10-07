package authoring

import (
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

func TestAuthoringCompositionUsesFrozenInputsAndSeparatesFictionalContext(t *testing.T) {
	in := operationInput{Kind: WritingVoice, Mode: Recommend, CandidateCount: 4, Guide: "kind guide", Purpose: "purpose", Prompt: "owner request", SourceContext: "accepted style context", ReferencePost: "sample prose", TargetID: "private-target", SessionID: "private-session", OperationID: "private-operation"}
	c := authoringComposition(in)
	inspection, err := llm.PreparedRequestInspection(llm.Request{System: authoringSystem, Composition: c})
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Status != llm.InspectionPrepared || inspection.IssuedAt != nil || inspection.Conditions != nil {
		t.Fatalf("fabricated execution: %+v", inspection)
	}
	if inspection.Fragments[0].Role != llm.InspectionRoleSystem || inspection.Fragments[0].Authorship != llm.FragmentAuthorshipCode || !strings.Contains(inspection.Fragments[0].MaterialRole, "fictional") {
		t.Fatal("fictional code contract was misclassified")
	}
	var actualUser strings.Builder
	requestText := ""
	guideCode := false
	for _, f := range inspection.Fragments {
		if f.Role == llm.InspectionRoleUser {
			actualUser.WriteString(f.Text)
		}
		if f.ID == "guide" {
			guideCode = f.Authorship == llm.FragmentAuthorshipCode
		}
		if f.ID == "request" {
			requestText = f.Text
			if f.Authorship != llm.FragmentAuthorshipAccount {
				t.Fatal("owner request authorship lost")
			}
		}
	}
	if !guideCode || requestText != `"owner request"` || actualUser.String() != modelMessage(in) {
		t.Fatal("inventory differs from actual frozen authoring message")
	}

	for _, f := range inspection.Fragments {
		if strings.Contains(f.Text, "private-") {
			t.Fatal("private target/session identifiers entered prompt inventory")
		}
	}
	in.Prompt = "changed current settings"
	if requestText != `"owner request"` {
		t.Fatal("prepared request followed mutable current state")
	}
	if len(RequestCompositions()) != 10 {
		t.Fatal("kind/mode inventory incomplete")
	}
}
