package store_test

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestItemGroupBoundsRefusedAtTemplateSave(t *testing.T) {
	s, raw, _ := setup(t)
	body := `<clip version="1" intro="b" caption="bold" outro="e"><group id="menu" min="1" max="2"><field id="name" label="Name"/></group><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
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
			body := "<clip version=\"1\" intro=\"b\" caption=\"bold\" outro=\"e\"><text id=\"intro\" kind=\"fixed\" role=\"hook\" basis=\"output-start\"/><text id=\"outro\" kind=\"fixed\" role=\"ending\" basis=\"output-end\"/>\n<group id=\"menu\" label=\"" + label + "\" min=\"2\" max=\"2\"><field id=\"name\" label=\"Name\" required=\"true\"/></group></clip>"
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
	body := "<clip version=\"1\" intro=\"b\" caption=\"bold\" outro=\"e\"><text id=\"intro\" kind=\"fixed\" role=\"hook\" basis=\"output-start\"/><text id=\"outro\" kind=\"fixed\" role=\"ending\" basis=\"output-end\"/>\n<group id=\"menu\" label=\"메뉴\"><field id=\"name\" label=\"Name\" required=\"true\"/></group></clip>"
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
	body := `<clip version="1" intro="b" caption="bold" outro="e"><text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/><group id="menu" label="메뉴"><field id="name" label="Name"/></group></clip>`
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
