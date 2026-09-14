package ai_test

import (
	"encoding/json"
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

func TestNativeWriterAdmitsAll49ObservationsAtTheSourceCeiling(t *testing.T) {
	for _, structured := range []bool{false, true} {
		t.Run(fmt.Sprint(structured), func(t *testing.T) {
			writer, models, _ := newService(t, "", structured)
			in := planningInput()
			in.Template = clip.Recipe{Name: "synthetic release", Preset: "restaurant", CopyStyles: []string{"clean"}, InformationFields: []clip.InformationField{{Label: "상호", Prompt: "가게 이름"}, {Label: "위치", Prompt: "어디"}, {Label: "place", Prompt: "where"}}}
			p := clip.Project{Disclosure: "ad", Answers: []clip.Answer{{Label: "상호", Text: "연남 김밥"}, {Label: "위치", Text: "서울 연남동"}, {Label: "place", Text: "fixture"}}}
			owned := clip.LegacyProjectComposition(p, in.Template)
			in.Composition = &owned
			in.Analyses = nil
			for i := 0; i < 20; i++ {
				duration := 61000
				if i == 19 {
					duration = 641000
				}
				a := clip.SourceAnalysis{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i), Fingerprint: fmt.Sprintf("%064x", i), Info: clip.MediaInfo{DurationMS: duration, Width: 1280, Height: 720, HasAudio: true}}, Filename: fmt.Sprintf("fixture-%02d.mp4", i)}}
				for start := 0; start < duration; start += 60000 {
					a.Segments = append(a.Segments, clip.Segment{StartMS: start, EndMS: min(start+60000, duration), Event: "synthetic scene", Action: "moves", Motion: "static", Subjects: []string{"test pattern"}, Speech: "synthetic speech", Quality: "usable", Scene: "scenery", Focal: clip.Point{X: .5, Y: .5}, Subject: clip.Region{X: .3, Y: .3, Width: .4, Height: .4}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable})
				}
				in.Analyses = append(in.Analyses, a)
			}
			cuts := []any{}
			for i := 0; i < 3; i++ {
				cuts = append(cuts, map[string]any{"id": fmt.Sprintf("fixture-cut-%d", i), "source_id": in.Analyses[0].Source.ID, "template_section_id": "footage", "group_id": "", "item_id": "", "start_ms": i * 5000, "end_ms": (i + 1) * 5000, "rate_permille": 1000, "focal": map[string]float64{"x": .5, "y": .5}, "volume": 1, "observation_refs": []string{clip.ObservationID(in.Analyses[0].Source.ID, 0)}})
			}
			models.response.Text = raw(map[string]any{"ratio": in.Ratio, "duration_ms": 15000, "cuts": cuts, "generated": []any{}})
			system, user := ai.BuildPlanPrompt(in, 200)
			var schema json.RawMessage
			if structured {
				schema = ai.CompositionPlanSchema()
			}
			request, _ := json.Marshal(struct {
				System, User string
				Schema       json.RawMessage
			}{system, user, schema})
			if strings.Count(user, `"observation_id"`) != 49 {
				t.Fatal("dropped observations to fit the budget")
			}
			if len(request) > llm.ClipPlanInputUnits-2048 {
				t.Fatalf("complete frozen input exceeds reserved limit: %d bytes (system %d, user %d, schema %d)", len(request), len(system), len(user), len(schema))
			}
			plan, _, err := writer.Plan(t.Context(), testRef(), in)
			if err != nil || len(models.calls) != 1 || len(plan.Portable.Observations) != 20 {
				t.Fatalf("lost bounded source inventory: %v, calls=%d", err, len(models.calls))
			}
		})
	}
}

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
		{StartMS: 0, EndMS: 7500, Event: "해물라면을 담는다", Subjects: []string{"해물라면"}, Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Scene: "food", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable},
		{StartMS: 7500, EndMS: 15000, Event: "치즈라면을 담는다", Subjects: []string{"치즈라면"}, Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Scene: "food", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable},
	}
	return in
}

func nativePlan() map[string]any {
	cuts, generated := []any{}, []any{}
	for i, id := range []string{"sea", "cheese"} {
		cutID := "cut-" + id
		observation := []string{clip.ObservationID("source", i)}
		text := []string{"해물라면 12,000원", "치즈라면 $12 per serving"}[i]
		cuts = append(cuts, map[string]any{"id": cutID, "source_id": "source", "template_section_id": "dish", "group_id": "menu", "item_id": id, "start_ms": i * 7500, "end_ms": (i + 1) * 7500, "rate_permille": 1000, "focal": map[string]any{"x": .5, "y": .5}, "volume": 1, "observation_refs": observation})
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
		if len(models.calls) != 1 || sizer.calls+sizer.fixed+sizer.cards+sizer.layouts != 0 || usage != models.response.Usage || s.CompositionPlanVersion() != 6 {
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
		// The recorded status decides for a v2 scene; the prose heuristic decides
		// only for a legacy record that never carried one.
		{"uncertain_status", func(in *clip.PlanningInput, _ map[string]any) {
			in.Analyses[0].Segments[0].Certainty = clip.CertaintyUncertain
		}, "item_uncertain"},
		// A v2 scene that says it is certain keeps its item even when its prose
		// happens to contain the legacy keyword.
		{"v2_prose_does_not_override_status", func(in *clip.PlanningInput, _ map[string]any) {
			in.Analyses[0].Segments[0].Quality = "uncertain"
			in.Analyses[0].Segments[0].Event = "음식을 담는다"
			in.Analyses[0].Segments[0].Subjects = []string{"접시"}
		}, "item_unassigned"},
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
		{"invented_element", func(_ *clip.PlanningInput, p map[string]any) { nativeGenerated(p, 0)["element_id"] = "cta" }, true},
		{"fixed_rewrite", func(_ *clip.PlanningInput, p map[string]any) { nativeGenerated(p, 0)["element_id"] = "sticker" }, true},
		{"unknown_section", func(_ *clip.PlanningInput, p map[string]any) { firstCut(p)["template_section_id"] = "invention" }, false},
		{"missing_cut_reference", func(_ *clip.PlanningInput, p map[string]any) { firstCut(p)["observation_refs"] = []string{} }, true},
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

// A v2 record covers its whole source, so an unobserved gap is refused as INPUT
// — before the writer is paid — rather than caught afterwards on a cut that
// crossed it (CLIP-10, CLIP-92).
func TestObservationGapIsRefusedBeforeTheWriterIsPaid(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	in.Analyses[0].Segments[0].EndMS = 7499
	s, models, _ := newService(t, raw(p), true)
	_, _, err := s.Plan(t.Context(), testRef(), in)
	d, ok := clip.DiagnosticFromError(err)
	if err == nil || len(models.calls) != 0 {
		t.Fatalf("a gapped observation reached the writer: %v calls=%d", err, len(models.calls))
	}
	if !ok || d.Check != "observe_coverage_gap" || d.Values["segment"] != 2 || d.Values["previous_end_ms"] != 7499 {
		t.Fatalf("lost the coverage diagnostic: %+v", d)
	}
}

// One observed scene may supply SEVERAL cuts, as distinct half-open ranges that
// touch but never overlap. Reordering them changes the timeline and nothing
// else: each keeps its own identity, evidence, item binding and ORIGINAL source
// timestamps (CLIP-7, CLIP-98, CDS-62).
func TestSameSceneSplitsKeepIdentityAndSourceTimeThroughReordering(t *testing.T) {
	build := func(reversed bool) (clip.PlanningInput, map[string]any) {
		in, p := nativeInput(), nativePlan()
		cuts := p["cuts"].([]any)
		sea := cuts[0].(map[string]any)
		second := map[string]any{}
		for k, v := range sea {
			second[k] = v
		}
		// Two cuts out of the SAME observed scene 0..7500: [0,3000) and
		// [3000,7500). Touching ends are adjacent, not overlapping.
		sea["end_ms"], second["start_ms"], second["end_ms"] = 3000, 3000, 7500
		second["id"] = "cut-sea-2"
		p["cuts"] = []any{sea, second, cuts[1]}
		generated := p["generated"].([]any)
		p["generated"] = []any{generated[0], generated[1]}
		if reversed {
			p["cuts"] = []any{second, sea, cuts[1]}
		}
		return in, p
	}
	for _, reversed := range []bool{false, true} {
		in, p := build(reversed)
		s, models, _ := newService(t, raw(p), true)
		plan, _, err := s.Plan(t.Context(), testRef(), in)
		if err != nil || len(models.calls) != 1 {
			t.Fatalf("same-scene split refused: %v", err)
		}
		ranges := map[string][2]int{}
		items := map[string]string{}
		for _, c := range plan.Portable.Cuts {
			ranges[c.ID], items[c.ID] = [2]int{c.StartMS, c.EndMS}, c.ItemID
		}
		if ranges["cut-sea"] != [2]int{0, 3000} || ranges["cut-sea-2"] != [2]int{3000, 7500} {
			t.Fatalf("original source timestamps changed: %v", ranges)
		}
		if items["cut-sea"] != "sea" || items["cut-sea-2"] != "sea" || items["cut-cheese"] != "cheese" {
			t.Fatalf("item bindings moved with the order: %v", items)
		}
		for _, c := range plan.Cuts {
			if evidence, covered := clip.CutEvidence(in.Analyses, c); !covered || len(evidence) != 1 {
				t.Fatalf("cut %s lost its single-scene evidence", c.ID)
			}
		}
	}
	// The same two ranges overlapping by one millisecond are refused outright.
	in, p := build(false)
	p["cuts"].([]any)[1].(map[string]any)["start_ms"] = 2999
	s, models, _ := newService(t, raw(p), true)
	delivered, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil || len(models.calls) != 1 || len(delivered.Cuts) != 2 || !hasNotice(delivered, "plan_source_overlap") || clip.ValidateSourceRanges(delivered, nil) != nil {
		t.Fatalf("overlapping cut not removed with a notice: %v %+v", err, delivered)
	}
}

// The writer receives the frozen narrative, the owner's exact facts, the
// complete observations, source metadata and the SERVER's own allowed-rate
// list — and nothing else. No pixels, no URL, and no authority over sound:
// the owner's source-sound setting is not in the request at all (CLIP-31,
// CLIP-100, CDS-68).
func TestWriterInputCarriesAllowedRatesAndNoAudioAuthority(t *testing.T) {
	in := nativeInput()
	// A 60 fps original earns the slow rates; the fixture's unmeasured source
	// earns only 1x and faster.
	in.Analyses[0].Source.Info.FrameRateNumerator, in.Analyses[0].Source.Info.FrameRateDenominator = 60, 1
	in.Analyses[0].Source.Info.DecodedFrames, in.Analyses[0].Source.Info.DecodedDurationMS = 900, 15000
	in.Analyses[0].Source.Info.CadenceVerified = true
	system, user := ai.BuildPlanPrompt(in, 200)
	var payload map[string]any
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	analysis := payload["analyses"].([]any)[0].(map[string]any)
	got := analysis["allowed_rate_permille"].([]any)
	want := clip.AllowedPlaybackRates(in.Analyses[0].Source.Info)
	if len(got) != len(want) {
		t.Fatalf("allowed rates lost: %v, want %v", got, want)
	}
	for i, rate := range want {
		if int(got[i].(float64)) != rate {
			t.Fatalf("allowed rates changed: %v, want %v", got, want)
		}
	}
	for _, forbidden := range []string{"retain", "original_audio", "original_sound", "source_audio", "http://", "https://", "object_key", "signature"} {
		if strings.Contains(strings.ToLower(system+user), forbidden) {
			t.Fatalf("the writer request carried %q", forbidden)
		}
	}
	// Each observed scene reaches the writer with its recorded status, so the
	// model can obey the selection rules it is given.
	segment := analysis["segments"].([]any)[0].(map[string]any)
	if segment["certainty"] != "certain" || segment["usability"] != "usable" || segment["action"] == nil || segment["motion"] == nil {
		t.Fatalf("observation status lost on the way to the writer: %v", segment)
	}
}

// Automatic assembly may not select footage the observer called unusable or
// unknown, and uncertain footage only at 1x. An unsuitable rate is NAMED, never
// quietly replaced (CLIP-99, CDS-68).
func TestUnusableAndUnknownFootageCannotBeSelectedAutomatically(t *testing.T) {
	for _, tc := range []struct {
		name, check          string
		certainty, usability string
		rate                 int
	}{
		{"unusable", "plan_cut_count", clip.CertaintyCertain, clip.UsabilityUnusable, 1000},
		{"unknown", "plan_cut_count", clip.CertaintyUnknown, clip.UsabilityUsable, 1000},
		{"uncertain at 1x", "", clip.CertaintyUncertain, clip.UsabilityUsable, 1000},
		{"uncertain sped up", "", clip.CertaintyUncertain, clip.UsabilityUsable, 2000},
		{"certain sped up", "", clip.CertaintyCertain, clip.UsabilityUsable, 2000},
		{"rate outside the source's own set", "plan_timeline", clip.CertaintyCertain, clip.UsabilityUsable, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := nativeInput(), nativePlan()
			// Scale the FOOTAGE by the rate so every case still produces the
			// same 15 s output: what is under test is the selection rule, not
			// whether the target happens to be reachable.
			scale := func(ms int) int { return ms * tc.rate / clip.RateUnitPermille }
			in.Analyses[0].Source.Info.DurationMS = scale(15000)
			for i := range in.Analyses[0].Segments {
				seg := &in.Analyses[0].Segments[i]
				seg.StartMS, seg.EndMS = scale(seg.StartMS), scale(seg.EndMS)
				seg.Certainty, seg.Usability = tc.certainty, tc.usability
			}
			for _, value := range p["cuts"].([]any) {
				c := value.(map[string]any)
				c["rate_permille"] = tc.rate
				c["start_ms"], c["end_ms"] = scale(c["start_ms"].(int)), scale(c["end_ms"].(int))
			}
			s, models, _ := newService(t, raw(p), true)
			_, _, err := s.Plan(t.Context(), testRef(), in)
			d, _ := clip.DiagnosticFromError(err)
			if tc.check == "" {
				if err != nil {
					t.Fatalf("selectable footage was refused: %v %+v", err, d)
				}
				return
			}
			if err == nil || d.Check != tc.check || len(models.calls) != 1 {
				t.Fatalf("unsuitable selection accepted or retried: %v %+v calls=%d", err, d, len(models.calls))
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

// Reconciliation stays inside the cut's OWN observed scene, so a cut can never
// be grown into the next item's footage to reach the target. The shortfall is
// reported instead — with no format retry, because insufficient footage is not
// a malformed response (CLIP-7, CLIP-95).
func TestReconciliationCannotReachAnotherItemsFootage(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	p["cuts"] = []any{p["cuts"].([]any)[0]}
	p["generated"] = []any{p["generated"].([]any)[0]}
	s, models, _ := newService(t, raw(p), true)
	_, _, err := s.Plan(t.Context(), testRef(), in)
	d, ok := clip.DiagnosticFromError(err)
	if err == nil || !ok || d.Check != "plan_timeline" || d.Phase != "timeline_grow" || len(models.calls) != 1 {
		t.Fatalf("a cut reached past its own scene: %v %+v calls=%d", err, d, len(models.calls))
	}
	// The same cut IS grown, up to its own scene's end, before the shortfall is
	// measured: the footage it may have, it takes.
	if d.Values["after_ms"] != 7500 || d.Values["remaining_ms"] != 7500 {
		t.Fatalf("the cut was not grown inside its own scene: %+v", d.Values)
	}
}

// A section's guide may ask for several cuts — the restaurant template's arrival
// splits an exterior and an entrance shot — and repetition is about which item a
// section speaks for, not about how many cuts it may hold (CLIP-59, CLIP-98).
// Only ORDER is the template's: the plan never returns to a section it left.
func TestNativeWriterAdmitsConsecutiveCutsInOneNonrepeatedSection(t *testing.T) {
	const body = `<clip version="1"><field id="fee" label="입장료"/><guide>긴 장면으로 설명</guide>` +
		`<scene id="arrival" scope="context"><text id="copy" kind="ai" role="caption" basis="cut">도착을 설명</text></scene>` +
		`<scene id="closing" scope="context"><text id="ending" kind="ai" role="caption" basis="cut">마무리를 설명</text></scene></clip>`
	sectionOf := func(p map[string]any, i int, id string) {
		p["cuts"].([]any)[i].(map[string]any)["template_section_id"] = id
	}
	for _, tc := range []struct {
		name     string
		sections [2]string
		valid    bool
	}{
		{"both_in_arrival", [2]string{"arrival", "arrival"}, true},
		{"arrival_then_closing", [2]string{"arrival", "closing"}, true},
		{"closing_then_arrival", [2]string{"closing", "arrival"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := nativeInput(), nativePlan()
			setNativeBody(&in, body)
			in.Composition.Inputs.Items = nil
			for i, id := range tc.sections {
				sectionOf(p, i, id)
				g := nativeGenerated(p, i)
				g["element_id"] = map[string]string{"arrival": "copy", "closing": "ending"}[id]
				g["text"] = []string{"입장료 12,000원", "안내된 비용은 12,000원"}[i]
				g["fact_refs"] = []any{nativeFact("fee", "", "")}
			}
			s, models, _ := newService(t, raw(p), true)
			plan, _, err := s.Plan(t.Context(), testRef(), in)
			if len(models.calls) != 1 {
				t.Fatalf("not one writer call: %d", len(models.calls))
			}
			if !tc.valid {
				d, ok := clip.DiagnosticFromError(err)
				if !errors.Is(err, llm.ErrBadOutput) || !ok || d.Check != "plan_timeline" {
					t.Fatalf("wanted a below-floor remainder, got %v (%+v)", err, d)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Cuts) != 2 || len(plan.Portable.Elements) != 2 {
				t.Fatalf("lost a cut or its copy: %d cuts, %+v", len(plan.Cuts), plan.Portable.Elements)
			}
			for i, cut := range plan.Portable.Cuts {
				if cut.SectionID != tc.sections[i] || cut.ItemID != "" {
					t.Fatalf("cut %d bound to %q/%q", i+1, cut.SectionID, cut.ItemID)
				}
			}
		})
	}
}
