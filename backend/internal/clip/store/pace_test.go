package store_test

import (
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"testing"
)

func TestTemplatePacePersistsAndPatchPresence(t *testing.T) {
	service, store, _ := setup(t)
	r := recipe()
	r.CopyStyles = []string{"clean", "simple"}
	r.CaptionPace = "rapid"
	created, err := service.CreateTemplate(t.Context(), "alice", r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetTemplate(t.Context(), "alice", created.ID)
	if err != nil || got.CaptionPace != "rapid" {
		t.Fatal(got, err)
	}
	name := "다른 이름"
	got, err = service.UpdateTemplate(t.Context(), "alice", created.ID, clip.TemplatePatch{Name: &name})
	if err != nil || got.CaptionPace != "rapid" {
		t.Fatal(got, err)
	}
	for _, pace := range []string{"steady", "", "rapid"} {
		got, err = service.UpdateTemplate(t.Context(), "alice", created.ID, clip.TemplatePatch{CaptionPace: &pace})
		if err != nil || got.CaptionPace != pace {
			t.Fatal(got, err)
		}
	}
	invalid := "fast"
	if _, err = service.UpdateTemplate(t.Context(), "alice", created.ID, clip.TemplatePatch{CaptionPace: &invalid}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal(err)
	}
	r.Name = "invalid"
	r.CaptionPace = invalid
	if _, err = service.CreateTemplate(t.Context(), "alice", r); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal(err)
	}
}
