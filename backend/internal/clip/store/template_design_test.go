package store_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func designedTemplate(t *testing.T, s interface {
	CreateTemplate(context.Context, string, clip.Recipe, ...clip.TemplateDesign) (clip.VideoTemplate, error)
}) clip.VideoTemplate {
	t.Helper()
	v, err := s.CreateTemplate(context.Background(), "alice", recipe(), clip.TemplateDesign{IntroPreset: "cover", OutroPreset: "e", CaptionStyles: []string{"neon", "word-pop"}})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func inputFor(templateID string) clip.ProjectInput {
	return clip.ProjectInput{Language: "ko", Title: "여행", VideoTemplateID: templateID, Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "sponsored", CompositionInputs: &clip.CompositionInputs{Values: map[string]string{"place": "서울"}, Items: map[string][]composition.Item{}}}
}

// CLIP-166: a template stores its design selection and answers with it; a selection the
// project's own rule refuses is refused here too.
func TestATemplateCarriesItsDesignSelection(t *testing.T) {
	projects, _, _ := setup(t)
	v := designedTemplate(t, projects)
	listed, err := projects.ListTemplates(context.Background(), "alice")
	if err != nil || len(listed) != 1 {
		t.Fatal(listed, err)
	}
	got := listed[0]
	if !reflect.DeepEqual(got.Design, clip.TemplateDesign{IntroPreset: "cover", OutroPreset: "e", CaptionStyles: []string{"neon", "word-pop"}}) {
		t.Fatalf("the template's selection was not kept: %+v", got.Design)
	}
	for _, bad := range []clip.TemplateDesign{{IntroPreset: "nowhere"}, {OutroPreset: "nowhere"}, {CaptionStyles: []string{"neon", "neon"}}} {
		if _, err := projects.CreateTemplate(context.Background(), "alice", recipe(), bad); err == nil {
			t.Fatalf("an invalid selection was saved: %+v", bad)
		}
	}
	styles := []string{"bold"}
	edited, err := projects.UpdateTemplate(context.Background(), "alice", v.ID, clip.TemplatePatch{CaptionStyles: &styles})
	if err != nil || !reflect.DeepEqual(edited.Design.CaptionStyles, styles) || edited.Design.IntroPreset != "cover" {
		t.Fatalf("a style edit did not keep the rest: %+v (%v)", edited.Design, err)
	}
}

// CLIP-168: a project created with the template takes its selection; a project switched to it
// takes it in the same write; 없음 and a later template edit move nothing.
func TestAProjectTakesTheTemplatesSelectionOnSelection(t *testing.T) {
	projects, _, _ := setup(t)
	ctx := context.Background()
	v := designedTemplate(t, projects)
	p, err := projects.CreateProject(ctx, "alice", inputFor(v.ID))
	if err != nil {
		t.Fatal(err)
	}
	if p.IntroPreset != "cover" || p.OutroPreset != "e" || !reflect.DeepEqual(p.CaptionStyles, []string{"neon", "word-pop"}) {
		t.Fatalf("the new project did not take the template's selection: %s %s %v", p.IntroPreset, p.OutroPreset, p.CaptionStyles)
	}
	// An explicit value still wins at creation.
	intro := "a"
	explicit := inputFor(v.ID)
	explicit.IntroPreset = &intro
	if q, err := projects.CreateProject(ctx, "alice", explicit); err != nil || q.IntroPreset != "a" || q.OutroPreset != "e" {
		t.Fatalf("an explicit preset lost to the template's: %s %s (%v)", q.IntroPreset, q.OutroPreset, err)
	}

	bare := inputFor("")
	bare.CompositionInputs = nil
	plain, err := projects.CreateProject(ctx, "alice", bare)
	if err != nil {
		t.Fatal(err)
	}
	defaults := composition.DefaultDesign()
	if plain.IntroPreset != defaults.Intro || plain.OutroPreset != defaults.Outro {
		t.Fatalf("a project with no template did not start at the defaults: %s %s", plain.IntroPreset, plain.OutroPreset)
	}
	id := v.ID
	switched, err := projects.UpdateProject(ctx, "alice", plain.ID, clip.ProjectPatch{VideoTemplateID: &id})
	if err != nil {
		t.Fatal(err)
	}
	if switched.IntroPreset != "cover" || switched.OutroPreset != "e" || !reflect.DeepEqual(switched.CaptionStyles, []string{"neon", "word-pop"}) {
		t.Fatalf("switching to the template did not take its selection: %+v", switched)
	}
	none := ""
	cleared, err := projects.UpdateProject(ctx, "alice", plain.ID, clip.ProjectPatch{VideoTemplateID: &none})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.IntroPreset != "cover" || !reflect.DeepEqual(cleared.CaptionStyles, []string{"neon", "word-pop"}) {
		t.Fatal("clearing to 없음 moved the selection", cleared.IntroPreset, cleared.CaptionStyles)
	}
	outro := "b"
	if _, err := projects.UpdateTemplate(ctx, "alice", v.ID, clip.TemplatePatch{OutroPreset: &outro}); err != nil {
		t.Fatal(err)
	}
	kept, err := projects.GetProject(ctx, "alice", p.ID)
	if err != nil || kept.OutroPreset != "e" {
		t.Fatal("a template edit restyled a project made with it", kept.OutroPreset, err)
	}
}
