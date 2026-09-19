package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestItemGroupBoundsRefusedAtTemplateSave(t *testing.T) {
	s, raw, _ := setup(t)
	body := `<clip version="1"><group id="menu" min="1" max="2"><field id="name" label="Name"/></group><text id="empty-hook" kind="fixed" role="hook"/><text id="empty-ending" kind="fixed" role="ending"/></clip>`
	template, err := s.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "menu", CompositionBody: body})
	if err != nil {
		t.Fatal(err)
	}
	for _, attrs := range []string{`min="-1"`, `max="1.5"`, `min="2" max="1"`, `max="21"`} {
		bad := strings.Replace(body, `min="1" max="2"`, attrs, 1)
		for _, update := range []bool{false, true} {
			if update {
				_, err = s.UpdateTemplate(t.Context(), "alice", template.ID, clip.TemplatePatch{CompositionBody: &bad})
			} else {
				_, err = s.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "invalid", CompositionBody: bad})
			}
			var problem *composition.Problem
			if !errors.As(err, &problem) || problem.Reason != "invalid_item_bounds" {
				t.Fatalf("%s update=%v: %v", attrs, update, err)
			}
		}
	}
	got, err := raw.GetTemplate(t.Context(), "alice", template.ID)
	if err != nil || got.CompositionBody != body {
		t.Fatal("refused template changed", err)
	}
}

func TestItemGroupMinimumRefusesQuoteBeforeWorkButAllowsDrafts(t *testing.T) {
	for name, label := range map[string]string{"named": "메뉴", "undeclared_name": ""} {
		t.Run(name, func(t *testing.T) {
			h := generationSetup(t)
			body := "<clip version=\"1\" intro=\"b\" caption=\"bold\" outro=\"e\"><text id=\"intro\" kind=\"fixed\" role=\"hook\"/><text id=\"outro\" kind=\"fixed\" role=\"ending\"/>\n<group id=\"menu\" label=\"" + label + "\" min=\"2\" max=\"2\"><field id=\"name\" label=\"Name\" required=\"true\"/></group></clip>"
			template, err := h.projects.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "menu", CompositionBody: body})
			if err != nil {
				t.Fatal(err)
			}
			p, err := h.projects.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "incomplete", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
			if err != nil {
				t.Fatal("empty draft refused", err)
			}
			name := label
			if name == "" {
				name = "menu"
			}
			for _, items := range [][]composition.Item{nil, {}, {{ID: "first", Values: map[string]string{"name": "파스타"}}}} {
				inputs := clip.CompositionInputs{Items: map[string][]composition.Item{"menu": items}}
				p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CompositionInputs: &inputs})
				if err != nil {
					t.Fatal("short draft refused", err)
				}
				_, err = h.service.Quote(t.Context(), "alice", p.ID, h.batch.ID, "p/o", "p/w")
				var problem *composition.Problem
				if !errors.As(err, &problem) || problem.Reason != "items_required" || problem.ElementID != name || problem.Line != 2 {
					t.Fatalf("wrong refusal: %+v", err)
				}
				assertNoQuoteWork(t, h)
			}
			inputs := clip.CompositionInputs{Items: map[string][]composition.Item{"menu": {{ID: "first", Values: map[string]string{"name": "파스타"}}, {ID: "second", Values: map[string]string{"name": "피자"}}}}}
			p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CompositionInputs: &inputs})
			if err != nil {
				t.Fatal(err)
			}
			if err = clip.RequiredAnswers(template, p, config.ClipCompositionLimits()); err != nil {
				t.Fatal("complete group refused", err)
			}
			inputs.Items["menu"][1].Values["name"] = ""
			p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CompositionInputs: &inputs})
			if err != nil {
				t.Fatal("blank item draft refused", err)
			}
			var problem *composition.Problem
			if err = clip.RequiredAnswers(template, p, config.ClipCompositionLimits()); !errors.As(err, &problem) || problem.Reason != "required_binding" {
				t.Fatal("item field requirement lost", err)
			}
			inputs.Items["menu"] = append(inputs.Items["menu"], composition.Item{ID: "third"})
			if _, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CompositionInputs: &inputs}); !errors.As(err, &problem) || problem.Reason != "item_limit" {
				t.Fatal("maximum ignored", err)
			}
		})
	}
}

// A template saved before CLIP-119 declares no minimum, so it must still load;
// the group holding a required field is refused only when generation is asked
// for, and the refusal names the group with the number it admits and the number
// given.
func TestRequiredFieldGroupAdmitsOneItemAtQuoteNotAtTemplateLoad(t *testing.T) {
	h := generationSetup(t)
	body := "<clip version=\"1\" intro=\"b\" caption=\"bold\" outro=\"e\"><text id=\"intro\" kind=\"fixed\" role=\"hook\"/><text id=\"outro\" kind=\"fixed\" role=\"ending\"/>\n<group id=\"menu\" label=\"메뉴\"><field id=\"name\" label=\"Name\" required=\"true\"/></group></clip>"
	template, err := h.projects.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "menu", CompositionBody: body})
	if err != nil {
		t.Fatal("legacy template refused at load", err)
	}
	p, err := h.projects.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "legacy", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal("empty draft refused", err)
	}
	_, err = h.service.Quote(t.Context(), "alice", p.ID, h.batch.ID, "p/o", "p/w")
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.Reason != "items_required" || problem.ElementID != "메뉴" {
		t.Fatalf("wrong refusal: %+v", err)
	}
	want := map[string]string{"element_id": "메뉴", "line": "2", "reason": "items_required", "label": "메뉴", "min": "1", "actual": "0"}
	if got := problem.FailureParams(); !maps.Equal(got, want) {
		t.Fatalf("refusal parameters %+v, want %+v", got, want)
	}
	assertNoQuoteWork(t, h)
	inputs := clip.CompositionInputs{Items: map[string][]composition.Item{"menu": {{ID: "first", Values: map[string]string{"name": "파스타"}}}}}
	if p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{CompositionInputs: &inputs}); err != nil {
		t.Fatal(err)
	}
	if err = clip.RequiredAnswers(template, p, config.ClipCompositionLimits()); err != nil {
		t.Fatal("one item refused", err)
	}
}

// An undeclared group whose fields are all optional still admits any number,
// the empty set included (CLIP-61).
func TestOptionalFieldGroupAdmitsNoItem(t *testing.T) {
	h := generationSetup(t)
	body := `<clip version="1"><text id="intro" kind="fixed" role="hook"/><text id="outro" kind="fixed" role="ending"/><group id="menu" label="메뉴"><field id="name" label="Name"/></group></clip>`
	template, err := h.projects.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "menu", CompositionBody: body})
	if err != nil {
		t.Fatal(err)
	}
	p, err := h.projects.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "optional", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	if err = clip.RequiredAnswers(template, p, config.ClipCompositionLimits()); err != nil {
		t.Fatal("empty optional group refused", err)
	}
}

// A whole-source binding made before generation is saved with the project and
// frozen into the attempt, so the writer receives it (CLIP-123, CLIP-69).
func TestWholeSourceBindingSurvivesSaveAndGeneration(t *testing.T) {
	h := generationSetup(t)
	ctx := context.Background()
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, compositionPlanner{h.planner}, compositionRenderer{h.renderer}, generationJobs{h.queue}, h.cfg, generationDeps(generationFinisher{h.store}, &quotePricing{}, nil))
	body := "<clip version=\"1\" intro=\"b\" caption=\"bold\" outro=\"e\"><text id=\"intro\" kind=\"fixed\" role=\"hook\"/><text id=\"outro\" kind=\"fixed\" role=\"ending\"/>\n<group id=\"menu\" label=\"고기\"><field id=\"name\" label=\"부위\" required=\"true\"/></group><repeat for=\"menu\"><scene id=\"cut\" scope=\"item\"><text id=\"copy\" kind=\"ai\" role=\"caption\" basis=\"cut\">설명</text></scene></repeat></clip>"
	template, err := legacyTemplate(t, h.store, "alice", "meat", body), error(nil)
	if err != nil {
		t.Fatal(err)
	}
	source := h.batch.Sources[0]
	inputs := clip.CompositionInputs{
		Items: map[string][]composition.Item{"menu": {{ID: "belly", Values: map[string]string{"name": "삼겹살"}}}},
		Associations: []clip.SourceAssociation{{
			GroupID: "menu", ItemID: "belly", SourceID: source.ID, Fingerprint: source.Fingerprint,
			StartMS: 0, EndMS: source.DurationMS,
		}},
	}
	// No observations exist yet, so this binding has nothing to be checked
	// against and must be accepted rather than refused.
	p, err := h.projects.UpdateProject(ctx, "alice", h.project.ID, clip.ProjectPatch{VideoTemplateID: &template.ID, CompositionInputs: &inputs})
	if err != nil {
		t.Fatal("a pre-generation binding was refused", err)
	}
	if got := p.Composition.Inputs.Associations; len(got) != 1 || got[0].ItemID != "belly" || got[0].StartMS != 0 || got[0].EndMS != source.DurationMS {
		t.Fatalf("the binding was not saved whole: %+v", got)
	}
	h.project = p
	jobID, err := accept(h, quote(t, h))
	if err != nil {
		t.Fatal(err)
	}
	j, err := h.jobs.GetByID(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct{ Composition *clip.ProjectComposition }
	if json.Unmarshal(j.Payload, &snapshot) != nil || snapshot.Composition == nil {
		t.Fatal(string(j.Payload))
	}
	frozen := snapshot.Composition.Inputs.Associations
	if len(frozen) != 1 || frozen[0].ItemID != "belly" || frozen[0].EndMS != source.DurationMS {
		t.Fatalf("the binding was not frozen into the attempt: %+v", frozen)
	}
}
