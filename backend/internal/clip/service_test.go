package clip_test

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

type memoryStore struct {
	clip.SourceStore
	clip.Store
	last  clip.VideoTemplate
	patch clip.TemplatePatch
}

func (s *memoryStore) InsertTemplate(_ context.Context, v clip.VideoTemplate) error {
	s.last = v
	return nil
}
func (s *memoryStore) GetTemplate(_ context.Context, user, id string) (clip.VideoTemplate, error) {
	if id != s.last.ID || user != s.last.UserID {
		return clip.VideoTemplate{}, clip.ErrNotFound
	}
	return s.last, nil
}
func (s *memoryStore) UpdateTemplate(_ context.Context, _, _ string, p clip.TemplatePatch, _ time.Time) (clip.VideoTemplate, error) {
	s.patch = p
	return s.last, nil
}

// A template is its name and its outline body (CLIP-4, CLIP-14): a create with
// no body is refused rather than minted as a template of some other kind, and
// the body is kept exactly as the owner wrote it.
func TestTemplateIsItsNameAndOutlineBody(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{}
	s := testProjects(store)
	for _, body := range []string{"", "  \n "} {
		if _, err := s.CreateTemplate(ctx, "owner", clip.Recipe{Name: "카페", CompositionBody: body}); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("a body-less template was created: %q %v", body, err)
		}
	}
	if store.last.ID != "" {
		t.Fatal("a refused template reached the store")
	}
	body := `<clip version="1"><field id="place" label="장소" required="true">  간판 그대로
</field></clip>`
	value, err := s.CreateTemplate(ctx, "owner", clip.Recipe{Name: strings.Repeat("한", 40), CompositionBody: body})
	if err != nil {
		t.Fatal(err)
	}
	if bytes, err := hex.DecodeString(value.ID); err != nil || len(bytes) != 16 {
		t.Fatal("invalid random id")
	}
	if value.CompositionBody != body || store.last.CompositionBody != body {
		t.Fatal("body rewritten")
	}
	var problem *composition.Problem
	if _, err := s.CreateTemplate(ctx, "owner", clip.Recipe{Name: "카페", CompositionBody: `<clip version="1"><scene id="s" scope="scene"/></clip>`}); !errors.As(err, &problem) {
		t.Fatalf("a body outside the outline grammar was accepted: %v", err)
	}
	for _, patch := range []clip.TemplatePatch{{Name: ptr("")}, {CompositionBody: ptr("")}} {
		if _, err := s.UpdateTemplate(ctx, "owner", value.ID, patch); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("invalid patch accepted: %+v %v", patch, err)
		}
	}
	if _, err := s.UpdateTemplate(ctx, "owner", value.ID, clip.TemplatePatch{Name: ptr(" normalized ")}); err != nil {
		t.Fatal(err)
	}
	if store.patch.Name == nil || *store.patch.Name != "normalized" {
		t.Fatal("presence or normalization lost")
	}
}

// The generation gate is the outline's own required values: the current
// template decides, and a blank value is missing (CLIP-5, CLIP-102).
func TestCurrentTemplateControlsRequiredAnswers(t *testing.T) {
	limits := clip.DefaultCompositionLimits()
	template := clip.VideoTemplate{ID: "t", Recipe: clip.Recipe{Name: "카페", CompositionBody: `<clip version="1"><field id="new" label="새 정보" required="true"/></clip>`}}
	project := clip.Project{Disclosure: "ad", Composition: &clip.ProjectComposition{Inputs: clip.CompositionInputs{Values: map[string]string{"new": " \n "}}}}
	var problem *composition.Problem
	if err := clip.RequiredAnswers(template, project, limits); !errors.As(err, &problem) || problem.Reason != "required_binding" {
		t.Fatal(err)
	}
	project.Composition.Inputs.Values["new"] = "새 정보 / exact English"
	if err := clip.RequiredAnswers(template, project, limits); err != nil {
		t.Fatal(err)
	}
	// With no template there is nothing declared to require (CLIP-5).
	if err := clip.RequiredAnswers(clip.VideoTemplate{}, clip.Project{}, limits); err != nil {
		t.Fatal(err)
	}
}

// Every one of the five phrases is a valid campaign type and nothing else is.
func TestDisclosureVocabulary(t *testing.T) {
	for _, id := range []string{"ad", "sponsored", "provided", "paid", "self"} {
		if !clip.ValidDisclosure(id) {
			t.Fatal(id)
		}
	}
	if clip.ValidDisclosure("") || clip.ValidDisclosure("blog") {
		t.Fatal("disclosure vocabulary")
	}
}
func ptr[T any](value T) *T { return &value }
