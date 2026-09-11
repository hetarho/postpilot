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
	value, err := s.CreateTemplate(ctx, "owner", clip.Recipe{Name: strings.Repeat("한", 40), Preset: "cafe", CutGuidance: "  Preserve exact\n안내  ", CopyStyles: []string{"clean", "memo", "bold"}})
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
	// A clip may not be approved without its campaign type and two verifiable
	// facts (CDS-1, CDS-5), so the fixture carries both.
	project := clip.Project{Disclosure: "ad", Answers: []clip.Answer{
		{Label: "old", Text: "saved"}, {Label: "new", Text: " \n "},
		{Label: "상호", Text: "연남 김밥"}, {Label: "위치", Text: "서울 연남동"},
	}}
	if err := clip.RequiredAnswers(template, project); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal(err)
	}
	project.Answers[1].Text = "새 정보 / exact English"
	if err := clip.RequiredAnswers(template, project); err != nil {
		t.Fatal(err)
	}
}

// CDS-1 and CDS-5 are checkable before a credit is reserved: the badge needs a
// campaign type and the screen needs two of 상호 · 위치 · 가격 · 메뉴.
func TestApprovalGateNamesWhatIsMissing(t *testing.T) {
	template := clip.VideoTemplate{}
	facts := func(pairs ...string) []clip.Answer {
		out := []clip.Answer{}
		for i := 0; i < len(pairs); i += 2 {
			out = append(out, clip.Answer{Label: pairs[i], Text: pairs[i+1]})
		}
		return out
	}
	if err := clip.ApprovalGate(template, clip.Project{Answers: facts("상호", "가게", "위치", "서울")}); !errors.Is(err, clip.ErrDisclosureRequired) {
		t.Fatal("a clip without a campaign type was approved:", err)
	}
	for _, disclosure := range []string{"편집", "AD", " ad"} {
		if err := clip.ApprovalGate(template, clip.Project{Disclosure: disclosure, Answers: facts("상호", "가게", "위치", "서울")}); !errors.Is(err, clip.ErrDisclosureRequired) {
			t.Fatalf("%q was accepted as a campaign type", disclosure)
		}
	}
	var missing *clip.MissingFactsError
	err := clip.ApprovalGate(template, clip.Project{Disclosure: "sponsored", Answers: facts("상호", " ", "위치", "서울", "영업", "매일")})
	if !errors.As(err, &missing) {
		t.Fatalf("one fact was enough: %v", err)
	}
	// The refusal names the empty labels, and 영업 is not one of the four.
	if strings.Join(missing.Labels, ",") != "상호,가격,메뉴" {
		t.Fatalf("missing labels = %v", missing.Labels)
	}
	for _, p := range []clip.Project{
		{Disclosure: "ad", Answers: facts("상호", "가게", "메뉴", "김밥")},
		{Disclosure: "self", Answers: facts("가격", "9900원", "메뉴", "김밥", "평점", "4.5")},
	} {
		if err := clip.ApprovalGate(template, p); err != nil {
			t.Fatalf("two facts were refused: %v", err)
		}
	}
	// Every one of the five phrases is a valid campaign type, and every CTA id
	// plus the empty one (meaning the preset's) is a valid CTA.
	for _, id := range []string{"ad", "sponsored", "provided", "paid", "self"} {
		if !clip.ValidDisclosure(id) {
			t.Fatal(id)
		}
	}
	for _, id := range []string{"", "blog", "place", "save"} {
		if !clip.ValidCTA(id) {
			t.Fatal(id)
		}
	}
	if clip.ValidCTA("subscribe") || clip.ValidPreset("") || !clip.ValidPreset("cafe") {
		t.Fatal("preset or CTA vocabulary")
	}
}
func ptr[T any](value T) *T { return &value }
