package store_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// A template carries no pace of its own (CLIP-14): a root pace attribute is
// part of the body the owner wrote and reaches no project. The project's pace
// starts at the shared default.
func TestTemplateCarriesNoPaceOfItsOwn(t *testing.T) {
	service, store, _ := setup(t)
	r := recipe()
	r.CompositionBody = `<clip version="1" pace="rapid"><field id="place" label="장소" required="true">어디였나요?</field></clip>`
	created, err := service.CreateTemplate(t.Context(), "alice", r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetTemplate(t.Context(), "alice", created.ID)
	if err != nil || got.CompositionBody != r.CompositionBody {
		t.Fatal(got, err)
	}
	p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "여행", VideoTemplateID: created.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil || p.CaptionPace != "" {
		t.Fatal("the template seeded the project's pace", p.CaptionPace, err)
	}
}
