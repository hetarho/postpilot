package ai_test

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestCompositionQualityEvidenceCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/composition-quality.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, Mutation, FirstItem, FirstText, FirstFallback string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			in, response := nativeInput(), nativePlan()
			if in.Analyses[0].Source.Info.HasAudio {
				t.Fatal("fixture is not silent")
			}
			for _, observed := range in.Analyses[0].Segments {
				if observed.Speech != "" {
					t.Fatal("invented speech")
				}
			}
			switch tc.Mutation {
			case "none":
			case "lookalike":
				in.Analyses[0].Segments[0].Subjects = []string{"접시", "라면"}
				in.Analyses[0].Segments[0].Event = "비슷한 면 요리를 담는다"
			case "missing_price":
				delete(in.Composition.Inputs.Items["menu"][0].Values, "price")
			case "experience":
				nativeGenerated(response, 0)["text"] = "먹어보니 맛있어요"
			case "global_price":
				nativeGenerated(response, 0)["fact_refs"] = []any{nativeFact("fee", "", "")}
			default:
				t.Fatal("unhandled corpus mutation", tc.Mutation)
			}
			writer, models, _ := newService(t, raw(response), true)
			plan, _, err := writer.Plan(t.Context(), testRef(), in)
			if err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 1 {
				t.Fatal("fixture used an extra repair call")
			}
			if plan.Portable.Cuts[0].ItemID != tc.FirstItem {
				t.Fatal("wrong subject association", plan.Portable.Cuts[0])
			}
			first := findNativeCopy(t, plan, "cut-sea")
			if tc.FirstText == "" {
				if first != nil || !hasFallback(plan, "cut-sea", tc.FirstFallback) {
					t.Fatal("unsupported copy survived", first, plan.Portable.Fallbacks)
				}
			} else if first == nil || first.Resolved.Text != tc.FirstText {
				t.Fatal("grounded copy changed", first)
			}
			second := findNativeCopy(t, plan, "cut-cheese")
			if second == nil || second.Resolved.Text != "치즈라면 $12 per serving" {
				t.Fatal("other dish changed", second)
			}
			for _, text := range plan.Portable.Elements {
				if text.Resolved.Element.ID == "optional" {
					t.Fatal("blank optional fact produced a placeholder")
				}
				for _, fact := range text.Resolved.Facts {
					if fact.GroupID != "menu" || fact.ItemID != text.Resolved.ItemID {
						t.Fatal("fact escaped its visible item", text)
					}
					matched := false
					for _, item := range in.Composition.Inputs.Items["menu"] {
						if item.ID == fact.ItemID && item.Values[fact.FieldID] == fact.Value && fact.Value != "" {
							matched = true
						}
					}
					if !matched {
						t.Fatal("fact value differs from its own authored item", fact)
					}
				}
				if text.Resolved.Element.Kind == "ai" {
					if len(text.Evidence) != 1 {
						t.Fatal("missing scene evidence", text)
					}
					index := 0
					if text.Resolved.ItemID == "cheese" {
						index = 1
					}
					e := text.Evidence[0]
					if e.SourceID != "source" || e.Fingerprint != in.Analyses[0].Source.Fingerprint || e.StartMS != index*7500 || e.EndMS != (index+1)*7500 {
						t.Fatal("caption points to the wrong scene", text)
					}
				}
			}
			if err := clip.ValidateCompositionEvidence(plan); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCompositionQualityManualReorderKeepsFrozenFactsAndExactText(t *testing.T) {
	in := nativeInput()
	writer, _, _ := newService(t, raw(nativePlan()), true)
	plan, _, err := writer.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := clip.EncodeEditPlan(plan, plan.Styles)
	if err != nil {
		t.Fatal(err)
	}
	analysis, _ := json.Marshal(in.Analyses)
	p := clip.Project{Ratio: plan.Ratio, EditPlan: encoded, Analysis: string(analysis), Composition: in.Composition, EditPlanRevision: 1}
	draft := clip.CorrectionFromPlan(plan)
	slices.Reverse(draft.Cuts)
	before := map[string]clip.CorrectionText{}
	for _, text := range draft.Elements {
		before[text.InstanceID] = text
	}
	corrected, styles, err := clip.ApplyCorrection(config.ClipRender(&config.Config{}), p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if err := clip.ValidateCompositionEvidence(corrected); err != nil {
		t.Fatal(err)
	}
	for _, text := range corrected.Portable.Elements {
		previous := before[text.Resolved.InstanceID]
		if text.Resolved.Text != previous.Text || text.Resolved.ItemID != previous.ItemID || !reflect.DeepEqual(text.Evidence, previous.Evidence) {
			t.Fatal("reordering changed caption identity or authored text", text)
		}
		if text.Resolved.Element.Kind == "ai" {
			want := 0
			if text.Resolved.CutID == "cut-sea" {
				want = 7500
			}
			if text.Resolved.StartMS < want || text.Resolved.EndMS > want+7500 {
				t.Fatal("caption did not follow its cut", text)
			}
		}
	}
	stored, err := clip.EncodeEditPlan(corrected, styles)
	if err != nil {
		t.Fatal(err)
	}
	// Editing the reusable source after persistence cannot rewrite this project's snapshot.
	in.Template.CompositionBody = `<clip version="1"/>`
	decoded, _, err := clip.DecodeEditPlan(stored)
	if err != nil || decoded.Portable.Snapshot.Body != nativeBody || !strings.Contains(stored, "12,000원") {
		t.Fatal("frozen content changed", err)
	}
}
