package memory

import (
	"context"
	"github.com/postpilot/backend/internal/llm"
	"strings"
	"testing"
)

func TestExtractionActualRequestCarriesPreparedCompositionWithoutAnotherCall(t *testing.T) {
	models := &fakeModels{answer: goodAnswer, structured: true}
	jobs := &fakeExtractions{}
	service, store := extractionService(t, models, &fakePosts{}, jobs)
	source := ExtractionSource{PostSlug: "post-1", Title: "title", Memo: "owner facts", Body: "canonical generated body"}
	if err := service.Extract(context.Background(), ExtractionJob{ID: "job", UserID: "owner", Model: "p/m", Source: source}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.requests) != 1 || len(store.inserted) != 0 {
		t.Fatal("inspection admitted extra extraction or saved facts")
	}
	got, err := llm.PreparedRequestInspection(models.requests[0])
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != llm.InspectionPrepared || got.IssuedAt != nil || got.Fragments[0].Text != models.requests[0].System || got.Fragments[4].Text != source.Memo || got.Fragments[6].MaterialRole != "canonical-post-prose-unconfirmed-origin" {
		t.Fatalf("incorrect frozen/material projection: %+v", got)
	}
	var user strings.Builder
	for _, f := range got.Fragments {
		if f.Role == llm.InspectionRoleUser {
			user.WriteString(f.Text)
		}
	}
	if user.String() != models.requests[0].Messages[0].Parts[0].Text {
		t.Fatal("inventory omitted actual envelope bytes")
	}
	if got.Output.Schema != string(MemoryCandidatesSchema()) || len(RequestCompositions()) != 1 {
		t.Fatal("output inventory differs from actual schema")
	}
}
