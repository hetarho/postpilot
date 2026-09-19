package ai_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/composition"
)

const nativeBody = `<clip version="1" intro="b" caption="bold" outro="e" accent="teal">
<field id="fee" label="입장료"/>
<group id="menu"><field id="name" label="메뉴" required="true"/><field id="price" label="가격"/><field id="extra" label="설명"/><field id="experience" label="경험"/></group>
<guide>음식을 긴 장면으로 차분하게 설명한다. 방문한 척하지 않는다.</guide>
<text id="authored" kind="fixed" role="badge" position="header" basis="whole">  &lt;직접 작성&gt; &amp; 그대로  </text>
<repeat for="menu"><scene id="dish" scope="item">
<text id="sticker" kind="fixed" role="info" basis="cut"><value field="menu.name"/>: <value field="menu.price"/></text>
<text id="optional" kind="fixed" role="info" basis="cut"><value field="menu.extra"/></text>
<text id="copy" kind="ai" role="caption" basis="cut">관찰한 <value field="menu.name"/>을 설명한다.</text>
</scene></repeat><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`

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

// A v2 record covers its whole source, so an unobserved gap is refused as INPUT
// — before the writer is paid — rather than caught afterwards on a cut that
// crossed it (CLIP-10, CLIP-92).
func TestObservationGapIsRefusedBeforeTheWriterIsPaid(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	in.Analyses[0].Segments[0].EndMS = 7499
	s, models, _ := newService(t, raw(p), true)
	_, _, err := s.Flow(t.Context(), testRef(), in)
	d, ok := clip.DiagnosticFromError(err)
	if err == nil || len(models.calls) != 0 {
		t.Fatalf("a gapped observation reached the writer: %v calls=%d", err, len(models.calls))
	}
	if !ok || d.Check != "observe_coverage_gap" || d.Values["segment"] != 2 || d.Values["previous_end_ms"] != 7499 {
		t.Fatalf("lost the coverage diagnostic: %+v", d)
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
	system, user := ai.BuildPlanPrompt(in, 200, clip.DefaultCompositionLimits())
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
			if _, _, err := s.Flow(t.Context(), testRef(), in); err == nil || len(models.calls) != 0 {
				t.Fatalf("provider called: %d %v", len(models.calls), err)
			}
		})
	}
	s, models, _ := newService(t, raw(nativePlan()), true)
	in := nativeInput()
	models.info.StructuredOutput = false
	if _, _, err := s.Flow(t.Context(), testRef(), in); !errors.Is(err, clip.ErrPricingUnavailable) || len(models.calls) != 0 {
		t.Fatalf("silently downgraded admission: %v", err)
	}
}

const admittedBody = `<clip version="1" intro="b" caption="bold" outro="e">
<group id="menu"><field id="name" label="메뉴" required="true"/></group>
<group id="extra"><field id="note" label="메모"/></group>
<guide>음식을 차분하게 설명한다.</guide>
<repeat for="menu"><scene id="dish" scope="item">
<text id="copy" kind="ai" role="caption" basis="cut">관찰한 <value field="menu.name"/>을 설명한다.</text>
</scene></repeat>
<repeat for="extra"><scene id="note" scope="item">
<text id="notecopy" kind="ai" role="caption" basis="cut">관찰한 장면을 설명한다.</text>
</scene></repeat>
<text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
