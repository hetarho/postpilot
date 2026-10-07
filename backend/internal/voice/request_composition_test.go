package voice

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

func TestWritingStyleComposersKeepAuthorshipSeparateFromMessageRole(t *testing.T) {
	samples := []Sample{{ID: "oldest", Body: "받아들인 문장입니다.\nhttps://signed.invalid/private?token=secret\n영업시간 11:00", PhotoKey: "secret-storage-key", UserID: "private-owner"}, {ID: "newest", Body: "새로운 문장입니다."}}
	req := analysisRequest(Fingerprint{}, samples)
	got, err := llm.PreparedRequestInspection(req)
	if err != nil {
		t.Fatal(err)
	}
	var actualUser strings.Builder
	var bodyIDs []string
	for _, f := range got.Fragments {
		if f.Role == llm.InspectionRoleUser {
			actualUser.WriteString(f.Text)
		}
		if f.MaterialRole == "accepted-owner-writing" {
			bodyIDs = append(bodyIDs, f.SourceRefs[0])
		}
	}
	if got.Fragments[1].Role != llm.InspectionRoleUser || got.Fragments[1].Authorship != llm.FragmentAuthorshipCode || len(bodyIDs) != 2 || bodyIDs[0] != "oldest" || bodyIDs[1] != "newest" || actualUser.String() != req.Messages[0].Parts[0].Text {
		t.Fatal("sample order, exact body or measured ownership changed")
	}

	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "secret-storage-key") || strings.Contains(string(raw), "private-owner") || strings.Contains(string(raw), "signed.invalid") || strings.Contains(string(raw), "영업시간") {
		t.Fatal("media storage or account details leaked")
	}
	candidates := candidateRequest(candidateInput{Count: 4, Directions: []string{"one", "two", "three", "four"}, Scene: "fictional scene", CompletionTokens: 2000})
	candidate, err := llm.PreparedRequestInspection(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Fragments[1].Authorship != llm.FragmentAuthorshipCode || candidate.Fragments[1].MaterialRole != "fictional-example-not-owner-fact" || candidate.Fragments[0].Text != candidates.System {
		t.Fatal("fictional candidate scene was treated as owner experience")
	}
	if candidate.Fragments[1].Text != candidates.Messages[0].Parts[0].Text {
		t.Fatal("candidate inventory differs from actual message")
	}
	check := checkComposition("frozen style", Prompt{Key: "catalog-photo", Photo: true, Scene: "example", Text: "question"})
	legacy, err := llm.PreparedCompositionInspection(check)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Fragments[0].Role != llm.InspectionRoleSystem || legacy.Fragments[0].Authorship != llm.FragmentAuthorshipAccount || legacy.Fragments[2].Text != "" || legacy.Fragments[2].SourceRefs[0] != "catalog-photo" || !strings.Contains(legacy.Activation, "admitted-only") {
		t.Fatal("legacy photo or style boundary misclassified")
	}
	if len(RequestCompositions()) != 3 {
		t.Fatal("style inventory incomplete")
	}
}
