package ai_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
)

const nativeBody = `<clip version="1" styles="memo" accent="teal">
<field id="fee" label="입장료"/>
<group id="menu"><field id="name" label="메뉴" required="true"/><field id="price" label="가격"/><field id="extra" label="설명"/><field id="experience" label="경험"/></group>
<guide>음식을 긴 장면으로 차분하게 설명한다. 방문한 척하지 않는다.</guide>
<text id="authored" kind="fixed" role="badge" position="header" basis="whole">  &lt;직접 작성&gt; &amp; 그대로  </text>
<repeat for="menu"><scene id="dish" scope="item">
<text id="sticker" kind="fixed" role="info" basis="cut"><value field="menu.name"/>: <value field="menu.price"/></text>
<text id="optional" kind="fixed" role="info" basis="cut"><value field="menu.extra"/></text>
<text id="copy" kind="ai" role="caption" basis="cut">관찰한 <value field="menu.name"/>을 설명한다.</text>
</scene></repeat></clip>`

func nativeInput() clip.PlanningInput {
	in := planningInput()
	in.Template = clip.Recipe{Name: "frozen native", CompositionBody: nativeBody}
	in.Composition = &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: 1, Body: nativeBody, TemplateID: "frozen-template"}, Inputs: clip.CompositionInputs{Values: map[string]string{"fee": "12,000원"}, Items: map[string][]composition.Item{"menu": {
		{ID: "sea", Values: map[string]string{"name": "해물라면", "price": "12,000원"}},
		{ID: "cheese", Values: map[string]string{"name": "치즈라면", "price": "$12 per serving"}},
	}}}}
	in.Answers = nil
	in.Analyses[0].Source.Info.DurationMS = 15000
	in.Analyses[0].Source.Info.HasAudio = false
	in.Analyses[0].Segments = []clip.Segment{
		{StartMS: 0, EndMS: 7500, Event: "해물라면을 담는다", Subjects: []string{"해물라면"}, Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Scene: "food"},
		{StartMS: 7500, EndMS: 15000, Event: "치즈라면을 담는다", Subjects: []string{"치즈라면"}, Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Scene: "food"},
	}
	return in
}

func nativePlan() map[string]any {
	cuts, generated := []any{}, []any{}
	for i, id := range []string{"sea", "cheese"} {
		cutID := "cut-" + id
		observation := []string{clip.ObservationID("source", i)}
		text := []string{"해물라면 12,000원", "치즈라면 $12 per serving"}[i]
		cuts = append(cuts, map[string]any{"id": cutID, "source_id": "source", "template_section_id": "dish", "group_id": "menu", "item_id": id, "start_ms": i * 7500, "end_ms": (i + 1) * 7500, "focal": map[string]any{"x": .5, "y": .5}, "volume": 1, "observation_refs": observation})
		generated = append(generated, map[string]any{"element_id": "copy", "cut_id": cutID, "text": text, "short_text": "", "keyword": "", "rows": []string{}, "short_rows": []string{}, "observation_refs": observation, "fact_refs": []any{nativeFact("name", "menu", id), nativeFact("price", "menu", id)}})
	}
	return map[string]any{"ratio": "vertical", "duration_ms": 15000, "cuts": cuts, "generated": generated}
}
func nativeFact(field, group, item string) map[string]any {
	return map[string]any{"field_id": field, "group_id": group, "item_id": item}
}
func nativeGenerated(value map[string]any, i int) map[string]any {
	return value["generated"].([]any)[i].(map[string]any)
}
func setNativeBody(in *clip.PlanningInput, body string) {
	in.Template.CompositionBody, in.Composition.Snapshot.Body = body, body
}
func findNativeCopy(t *testing.T, plan clip.EditPlan, cut string) *clip.PortableText {
	t.Helper()
	for i := range plan.Portable.Elements {
		text := &plan.Portable.Elements[i]
		if text.Resolved.Element.ID == "copy" && text.Resolved.CutID == cut {
			return text
		}
	}
	return nil
}
func hasFallback(plan clip.EditPlan, cut, reason string) bool {
	return slices.ContainsFunc(plan.Portable.Fallbacks, func(f clip.CopyFallback) bool { return f.CutID == cut && f.Reason == reason })
}

func TestNativeWriterOneCallPreservesAuthoredContentAndEvidence(t *testing.T) {
	for _, structured := range []bool{false, true} {
		s, models, sizer := newService(t, raw(nativePlan()), structured)
		in := nativeInput()
		if err := s.ValidatePreparation(testRef(), in, []clip.AnalysisSource{in.Analyses[0].Source}); err != nil {
			t.Fatal(err)
		}
		plan, usage, err := s.Plan(t.Context(), testRef(), in)
		if err != nil {
			t.Fatal(err)
		}
		if len(models.calls) != 1 || sizer.calls+sizer.fixed+sizer.cards+sizer.layouts != 0 || usage != models.response.Usage || s.CompositionPlanVersion() != 5 {
			t.Fatal("native stage repeated or called legacy compositor")
		}
		if plan.Portable.Snapshot.Body != nativeBody || plan.Hook != "" || plan.CTA != "" || plan.Disclosure != "" || len(plan.Cuts[0].Copies) != 0 {
			t.Fatalf("injected legacy content: %+v", plan)
		}
		if len(plan.Portable.Elements) != 5 {
			t.Fatalf("unexpected visible elements: %+v", plan.Portable.Elements)
		}
		if plan.Portable.Elements[0].Resolved.Text != "  <직접 작성> & 그대로  " {
			t.Fatal("authored whitespace rewritten")
		}
		for i, id := range []string{"sea", "cheese"} {
			text := findNativeCopy(t, plan, "cut-"+id)
			if text == nil || text.Resolved.ItemID != id || text.Resolved.Facts[1].ItemID != id || len(text.Evidence) != 1 || text.Evidence[0].StartMS != i*7500 || text.Evidence[0].EndMS != (i+1)*7500 || text.Evidence[0].Fingerprint != in.Analyses[0].Source.Fingerprint {
				t.Fatalf("lost scoped evidence: %+v", text)
			}
		}
		request := models.calls[0]
		if !strings.Contains(request.System, "viewpoint") || strings.Contains(request.System, "in first person") || request.MaxTokens != 32768 || (len(request.JSONSchema) > 0) != structured {
			t.Fatal("native request contract")
		}
		if structured && string(request.JSONSchema) != string(ai.CompositionPlanSchema()) {
			t.Fatal("legacy schema")
		}
		if !strings.Contains(request.Messages[0].Parts[0].Text, `"observation_id":"source/0"`) || !strings.Contains(request.Messages[0].Parts[0].Text, `"item_groups"`) {
			t.Fatal("missing source references or grouped inputs")
		}
		encoded, err := clip.EncodeEditPlan(plan, plan.Styles)
		if err != nil {
			t.Fatal(err)
		}
		decoded, _, err := clip.DecodeEditPlan(encoded)
		if err != nil || !reflect.DeepEqual(decoded.Portable, plan.Portable) {
			t.Fatalf("portable round trip: %v", err)
		}
	}
}

func TestNativeWriterRejectsCrossItemClaimsWithoutExtraCalls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*clip.PlanningInput, map[string]any)
		reason string
	}{
		{"other_item_fact", func(_ *clip.PlanningInput, p map[string]any) {
			nativeGenerated(p, 0)["fact_refs"] = []any{nativeFact("price", "menu", "cheese")}
		}, "unavailable_scoped_fact"},
		{"same_number_global", func(_ *clip.PlanningInput, p map[string]any) {
			nativeGenerated(p, 0)["fact_refs"] = []any{nativeFact("fee", "", "")}
		}, "unavailable_scoped_fact"},
		{"other_currency", func(_ *clip.PlanningInput, p map[string]any) { nativeGenerated(p, 0)["text"] = "해물라면 $12" }, "unsupported_number_unit"},
		{"borrowed_identity", func(_ *clip.PlanningInput, p map[string]any) {
			nativeGenerated(p, 0)["text"] = "치즈라면 12,000원"
		}, "cross_item_identity"},
		{"invented_experience", func(_ *clip.PlanningInput, p map[string]any) {
			nativeGenerated(p, 0)["text"] = "먹어보니 맛있어요"
		}, "unsupported_experience"},
		{"missing_price", func(in *clip.PlanningInput, _ map[string]any) {
			delete(in.Composition.Inputs.Items["menu"][0].Values, "price")
		}, "unavailable_scoped_fact"},
		{"other_observation", func(_ *clip.PlanningInput, p map[string]any) {
			nativeGenerated(p, 0)["observation_refs"] = []string{"source/1"}
		}, "missing_scene_evidence"},
		{"unassigned_generic", func(in *clip.PlanningInput, _ map[string]any) {
			in.Analyses[0].Segments[0].Event = "음식을 담는다"
			in.Analyses[0].Segments[0].Subjects = []string{"접시"}
		}, "item_unassigned"},
		{"uncertain_match", func(in *clip.PlanningInput, _ map[string]any) { in.Analyses[0].Segments[0].Quality = "uncertain" }, "item_uncertain"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := nativeInput(), nativePlan()
			tc.mutate(&in, p)
			s, models, _ := newService(t, raw(p), true)
			plan, _, err := s.Plan(t.Context(), testRef(), in)
			if err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 1 || findNativeCopy(t, plan, "cut-sea") != nil || !hasFallback(plan, "cut-sea", tc.reason) {
				t.Fatalf("unsafe copy/fallback: %+v", plan.Portable)
			}
			if findNativeCopy(t, plan, "cut-cheese") == nil {
				t.Fatal("unrelated item lost")
			}
		})
	}
}

func TestNativeWriterIgnoresProposedItemAndSupportsOwnerBinding(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	firstCut(p)["item_id"] = "cheese"
	in.Analyses[0].Segments[0].Event = "음식을 담는다"
	in.Analyses[0].Segments[0].Subjects = []string{"접시"}
	in.Composition.Inputs.Associations = []clip.SourceAssociation{{GroupID: "menu", ItemID: "sea", SourceID: "source", Fingerprint: in.Analyses[0].Source.Fingerprint, StartMS: 0, EndMS: 7500}}
	s, _, _ := newService(t, raw(p), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Portable.Cuts[0].ItemID != "sea" || findNativeCopy(t, plan, "cut-sea") == nil {
		t.Fatal("model identity beat owner evidence")
	}
}

func TestNativeWriterRespectsItemOrderAndRejectsStructuralInventions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*clip.PlanningInput, map[string]any)
		valid  bool
	}{
		{"reordered_items_and_cuts", func(in *clip.PlanningInput, p map[string]any) {
			slices.Reverse(in.Composition.Inputs.Items["menu"])
			slices.Reverse(p["cuts"].([]any))
		}, true},
		{"reordered_cuts_only", func(_ *clip.PlanningInput, p map[string]any) { slices.Reverse(p["cuts"].([]any)) }, false},
		{"invented_element", func(_ *clip.PlanningInput, p map[string]any) { nativeGenerated(p, 0)["element_id"] = "cta" }, false},
		{"fixed_rewrite", func(_ *clip.PlanningInput, p map[string]any) { nativeGenerated(p, 0)["element_id"] = "sticker" }, false},
		{"unknown_section", func(_ *clip.PlanningInput, p map[string]any) { firstCut(p)["template_section_id"] = "invention" }, false},
		{"source_gap", func(in *clip.PlanningInput, _ map[string]any) { in.Analyses[0].Segments[0].EndMS = 7499 }, false},
		{"missing_cut_reference", func(_ *clip.PlanningInput, p map[string]any) { firstCut(p)["observation_refs"] = []string{} }, false},
		{"case_key", func(_ *clip.PlanningInput, p map[string]any) {
			firstCut(p)["Source_ID"] = "source"
			delete(firstCut(p), "source_id")
		}, false},
		{"null", func(_ *clip.PlanningInput, p map[string]any) { nativeGenerated(p, 0)["text"] = nil }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := nativeInput(), nativePlan()
			tc.mutate(&in, p)
			s, models, _ := newService(t, raw(p), true)
			_, _, err := s.Plan(t.Context(), testRef(), in)
			if (err == nil) != tc.valid {
				t.Fatalf("valid %v error %v", tc.valid, err)
			}
			if len(models.calls) != 1 {
				t.Fatal("not one writer call")
			}
			if !tc.valid && !errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("wrong failure %v", err)
			}
		})
	}
}

func TestNativeWriterKeepsGroundedAlternativeWithoutRepairCall(t *testing.T) {
	p := nativePlan()
	nativeGenerated(p, 0)["short_text"] = "12,000원"
	s, models, _ := newService(t, raw(p), true)
	plan, _, err := s.Plan(t.Context(), testRef(), nativeInput())
	if err != nil {
		t.Fatal(err)
	}
	text := findNativeCopy(t, plan, "cut-sea")
	if len(models.calls) != 1 || len(text.Alternatives) != 1 || text.Alternatives[0].Text != "12,000원" {
		t.Fatalf("%+v", text)
	}
	nativeGenerated(p, 0)["text"] = "해물라면 15,000원"
	s, models, _ = newService(t, raw(p), true)
	plan, _, err = s.Plan(t.Context(), testRef(), nativeInput())
	if err != nil {
		t.Fatal(err)
	}
	text = findNativeCopy(t, plan, "cut-sea")
	if len(models.calls) != 1 || text.Resolved.Text != "12,000원" || !hasFallback(plan, "cut-sea", "unsupported_number_unit") {
		t.Fatalf("%+v", text)
	}
}

func TestNativeWriterAllowsExplicitContextAndNoAuthoredFurniture(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	setNativeBody(&in, `<clip version="1"><field id="fee" label="입장료"/><guide>긴 장면으로 설명</guide><repeat for="scenes"><scene id="context" scope="context"><text id="copy" kind="ai" role="caption" basis="cut">프로젝트 정보</text></scene></repeat></clip>`)
	in.Composition.Inputs.Items = nil
	for _, c := range p["cuts"].([]any) {
		c.(map[string]any)["template_section_id"] = "context"
	}
	for i, text := range []string{"입장료 12,000원", "안내된 비용은 12,000원"} {
		nativeGenerated(p, i)["text"] = text
		nativeGenerated(p, i)["fact_refs"] = []any{nativeFact("fee", "", "")}
	}
	s, _, _ := newService(t, raw(p), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Portable.Elements) != 2 || plan.Portable.Elements[0].Scope != "context" || plan.Portable.Cuts[0].ItemID != "" {
		t.Fatalf("%+v", plan.Portable)
	}
	nativeGenerated(p, 0)["text"] = "이 메뉴 12,000원"
	s, _, _ = newService(t, raw(p), true)
	plan, _, err = s.Plan(t.Context(), testRef(), in)
	if err != nil || !hasFallback(plan, "cut-sea", "context_item_claim") {
		t.Fatalf("%+v %v", plan.Portable, err)
	}
	setNativeBody(&in, `<clip version="1"><guide>긴 장면으로 보여 준다</guide></clip>`)
	in.Composition.Inputs.Values = nil
	for _, c := range p["cuts"].([]any) {
		c.(map[string]any)["template_section_id"] = ""
	}
	p["generated"] = []any{}
	s, _, _ = newService(t, raw(p), true)
	plan, _, err = s.Plan(t.Context(), testRef(), in)
	if err != nil || len(plan.Portable.Elements) != 0 {
		t.Fatalf("bare footage: %+v %v", plan.Portable, err)
	}
}

func TestNativeWriterBoundsAndFrozenAdmissionBeforeProvider(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*clip.PlanningInput)
	}{
		{"snapshot_mismatch", func(in *clip.PlanningInput) { in.Template.CompositionBody = `<clip version="1"/>` }},
		{"missing_required", func(in *clip.PlanningInput) { delete(in.Composition.Inputs.Items["menu"][0].Values, "name") }},
		{"expanded_input_budget", func(in *clip.PlanningInput) {
			body := `<clip version="1"><group id="menu">`
			values := map[string]string{}
			for i := range 10 {
				id := fmt.Sprintf("f%d", i)
				body += `<field id="` + id + `" label="항목"/>`
				values[id] = strings.Repeat("한", 500)
			}
			body += `</group><repeat for="scenes"><scene id="shot" scope="scene"/></repeat></clip>`
			setNativeBody(in, body)
			in.Composition.Inputs.Values = nil
			in.Composition.Inputs.Items["menu"] = nil
			for i := range 20 {
				in.Composition.Inputs.Items["menu"] = append(in.Composition.Inputs.Items["menu"], composition.Item{ID: fmt.Sprintf("item-%d", i), Values: values})
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, models, _ := newService(t, raw(nativePlan()), true)
			in := nativeInput()
			tc.mutate(&in)
			if err := s.ValidatePreparation(testRef(), in, []clip.AnalysisSource{in.Analyses[0].Source}); err == nil {
				t.Fatal("admitted unsupported inputs")
			}
			if _, _, err := s.Plan(t.Context(), testRef(), in); err == nil || len(models.calls) != 0 {
				t.Fatalf("provider called: %d %v", len(models.calls), err)
			}
		})
	}
	s, models, _ := newService(t, raw(nativePlan()), true)
	in := nativeInput()
	models.info.StructuredOutput = false
	if _, _, err := s.Plan(t.Context(), testRef(), in); !errors.Is(err, clip.ErrPricingUnavailable) || len(models.calls) != 0 {
		t.Fatalf("silently downgraded admission: %v", err)
	}
}

func TestNativeWriterRepetitionAndAuthoredRowRoles(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	setNativeBody(&in, `<clip version="1"><guide>긴 장면으로 보여 준다</guide><repeat for="scenes"><scene id="dish" scope="scene"><text id="copy" kind="ai" role="caption" basis="cut">관찰 설명</text></scene></repeat></clip>`)
	in.Composition.Inputs = clip.CompositionInputs{}
	for i := range 2 {
		nativeGenerated(p, i)["text"] = "접시를 담는다"
		nativeGenerated(p, i)["fact_refs"] = []any{}
	}
	s, models, _ := newService(t, raw(p), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil || len(models.calls) != 1 || len(plan.Portable.Elements) != 1 || !hasFallback(plan, "cut-cheese", "repeated_copy") {
		t.Fatalf("repeated promotional filler: %+v %v", plan.Portable, err)
	}
	setNativeBody(&in, `<clip version="1"><field id="fee" label="입장료"/><guide>긴 장면으로 보여 준다</guide><text id="card" kind="ai" role="hook" basis="output-start" start="0" end="3"><row role="title">관찰한 장소</row><row role="body">입장료</row></text></clip>`)
	in.Composition.Inputs.Values = map[string]string{"fee": "12,000원"}
	for _, c := range p["cuts"].([]any) {
		c.(map[string]any)["template_section_id"] = ""
	}
	g := nativeGenerated(p, 0)
	g["element_id"] = "card"
	g["cut_id"] = ""
	g["text"] = ""
	g["rows"] = []string{"영상 속 공간", "12,000원"}
	g["fact_refs"] = []any{nativeFact("fee", "", "")}
	p["generated"] = []any{g}
	s, _, _ = newService(t, raw(p), true)
	plan, _, err = s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	e := plan.Portable.Elements[0].Resolved
	if len(plan.Portable.Elements) != 1 || e.Rows[0].Role != "title" || e.Rows[1].Role != "body" || e.StartMS != 0 || e.EndMS != 3000 {
		t.Fatalf("authored rows/timing lost: %+v", e)
	}
}

func TestNativeWriterRechecksIdentityAfterTimelineGrowth(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	// One item section initially selects only its named scene. Extending to
	// the approved duration reaches a different item, so every dependent fact
	// and caption must lose its binding instead of leaking over the new range.
	p["cuts"] = []any{p["cuts"].([]any)[0]}
	p["generated"] = []any{p["generated"].([]any)[0]}
	s, _, _ := newService(t, raw(p), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if plan.DurationMS != 15000 || plan.Portable.Cuts[0].ItemID != "" || findNativeCopy(t, plan, "cut-sea") != nil || !hasFallback(plan, "cut-sea", "item_binding_conflict") {
		t.Fatalf("identity persisted across new footage: %+v", plan.Portable)
	}
}
