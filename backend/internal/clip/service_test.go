package clip_test

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
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
func TestTemplateValidationUsesScalarsAndPreservesExactGuidance(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{}
	s := clip.NewService(store, config.ClipLimits())
	value, err := s.CreateTemplate(ctx, "owner", clip.Recipe{Name: strings.Repeat("한", 40), CutGuidance: "  Preserve exact\n안내  ", CopyStyles: []string{"clean", "memo", "bold"}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes, err := hex.DecodeString(value.ID); err != nil || len(bytes) != 16 {
		t.Fatal("invalid random id")
	}
	if value.CutGuidance != "  Preserve exact\n안내  " {
		t.Fatal("guidance rewritten")
	}
	// An approved set must keep 깔끔하게 (every CDS fallback lands on it), hold no
	// duplicate and name none of the retired style ids.
	for _, patch := range []clip.TemplatePatch{{Name: ptr("")}, {CutGuidance: ptr(strings.Repeat("한", 4001))}, {Accent: ptr("arbitrary")}, {CopyStyles: ptr([]string{})}, {CopyStyles: ptr([]string{"memo", "bold"})}, {CopyStyles: ptr([]string{"clean", "clean"})}, {CopyStyles: ptr([]string{"clean", "diary"})}, {InformationFields: ptr([]clip.InformationField{{Label: "x", Prompt: " "}})}} {
		if _, err := s.UpdateTemplate(ctx, "owner", value.ID, patch); !errors.Is(err, clip.ErrInvalid) {
			t.Fatalf("invalid patch accepted: %+v %v", patch, err)
		}
	}
	if _, err := s.UpdateTemplate(ctx, "owner", value.ID, clip.TemplatePatch{Name: ptr(" normalized ")}); err != nil {
		t.Fatal(err)
	}
	if store.patch.Name == nil || *store.patch.Name != "normalized" || store.patch.CopyStyles != nil {
		t.Fatal("presence or normalization lost")
	}
}
func TestCurrentTemplateControlsRequiredAnswers(t *testing.T) {
	template := clip.VideoTemplate{Recipe: clip.Recipe{InformationFields: []clip.InformationField{{Label: "new"}}}}
	project := clip.Project{Answers: []clip.Answer{{Label: "old", Text: "saved"}, {Label: "new", Text: " \n "}}}
	if err := clip.RequiredAnswers(template, project); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal(err)
	}
	project.Answers[1].Text = "새 정보 / exact English"
	if err := clip.RequiredAnswers(template, project); err != nil {
		t.Fatal(err)
	}
}
func ptr[T any](value T) *T { return &value }
