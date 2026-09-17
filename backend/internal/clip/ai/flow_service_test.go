package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// A template of the grammar every clip is written against today: fixed regions,
// fields, a group and a guide — and no scene, so nothing in it admits or
// forbids footage (CLIP-136).
const flowBody = `<clip version="1" intro="b" caption="bold" outro="e" accent="teal">
<field id="place" label="상호" required="true">가게 이름</field>
<group id="menu" label="메뉴" min="1"><field id="name" label="메뉴" required="true">메뉴 이름</field><field id="price" label="가격">가격</field></group>
<guide>음식을 차분하게 보여준다.</guide>
<text id="badge" kind="fixed" role="badge" position="header" basis="whole">직접 작성</text>
<text id="hook" kind="fixed" role="hook" basis="output-start" start="0" end="2"><row><value field="place"/></row></text>
<text id="ending" kind="fixed" role="ending" basis="output-end" start="-2" end="0"><row>또 갈래요</row></text></clip>`

func flowInput() clip.PlanningInput {
	in := planningInput()
	in.Template = clip.Recipe{Name: "flow template", CompositionBody: flowBody}
	in.Answers = nil
	in.Composition = &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Version: 1, Body: flowBody, TemplateID: "flow-template"},
		Inputs: clip.CompositionInputs{
			Values: map[string]string{"place": "성수 곱창"},
			Items: map[string][]composition.Item{"menu": {
				{ID: "sea", Values: map[string]string{"name": "해물라면", "price": "12,000원"}},
				{ID: "cheese", Values: map[string]string{"name": "치즈라면", "price": "13,000원"}},
			}},
			Associations: []clip.SourceAssociation{{GroupID: "menu", ItemID: "sea", SourceID: "source", Fingerprint: "frozen-fingerprint", StartMS: 0, EndMS: 7500}},
		}}
	in.SourceAudio = []clip.SourceAudioSetting{{SourceID: "source", Fingerprint: "frozen-fingerprint", RetainOriginal: true}}
	in.TargetDurationMS = 22500
	in.Analyses[0].Source.Info.DurationMS = 30000
	in.Analyses[0].Segments = []clip.Segment{
		{StartMS: 0, EndMS: 15000, Event: "해물라면을 담는다", Subjects: []string{"해물라면"}, Speech: "맛있겠다", Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Scene: "food", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable},
		{StartMS: 15000, EndMS: 30000, Event: "치즈라면을 담는다", Subjects: []string{"치즈라면"}, Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Scene: "food", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable},
	}
	return in
}

func flowCut(id string, start, end, rate int) map[string]any {
	return map[string]any{"id": id, "source_id": "source", "start_ms": start, "end_ms": end, "rate_permille": rate,
		"focal": map[string]any{"x": .5, "y": .5}, "volume": 1, "observation_refs": []string{clip.ObservationID("source", start/15000)}}
}
func flowResponse(cuts ...map[string]any) string {
	values := []any{}
	for _, c := range cuts {
		values = append(values, c)
	}
	return raw(map[string]any{"ratio": "vertical", "duration_ms": 22500, "cuts": values})
}
func defaultFlow() string {
	return flowResponse(flowCut("cut-one", 0, 7500, 1000), flowCut("cut-two", 7500, 15000, 1000), flowCut("cut-three", 15000, 22500, 1000))
}

func flowRequest(t *testing.T, in clip.PlanningInput, response string) (clip.EditPlan, map[string]any, string) {
	t.Helper()
	s, models, _ := newService(t, response, true)
	plan, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatal("the flow is one writing call", len(models.calls))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(models.calls[0].Messages[0].Parts[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	return plan, payload, models.calls[0].System
}

func TestFlowRequestCarriesWhatTheOrderIsMadeFrom(t *testing.T) {
	in := flowInput()
	in.Instruction = "고기 장면을 먼저 보여줘"
	_, payload, system := flowRequest(t, in, defaultFlow())
	for _, key := range []string{"project_instruction", "template_guide", "global_values", "item_groups", "source_order", "item_hints", "analyses", "ratio", "target_duration_ms", "fade_ms"} {
		if _, ok := payload[key]; !ok {
			t.Fatal("the request lost " + key)
		}
	}
	if payload["project_instruction"] != in.Instruction || !strings.Contains(payload["template_guide"].(string), "차분하게") {
		t.Fatal("the instruction or the template's own guide is missing", payload["project_instruction"], payload["template_guide"])
	}
	if order := payload["source_order"].([]any); len(order) != 1 || order[0] != "source" {
		t.Fatal("the sources were not offered in the order they were arranged in", order)
	}
	hints := payload["item_hints"].([]any)
	if len(hints) != 1 || hints[0].(map[string]any)["item_id"] != "sea" || hints[0].(map[string]any)["end_ms"].(float64) != 7500 {
		t.Fatal("the owner's item hint did not reach the writer", hints)
	}
	if payload["global_values"].(map[string]any)["place"] != "성수 곱창" || len(payload["item_groups"].(map[string]any)["menu"].([]any)) != 2 {
		t.Fatal("the facts did not reach the writer", payload["global_values"], payload["item_groups"])
	}
	analysis := payload["analyses"].([]any)[0].(map[string]any)
	if analysis["retains_original_audio"] != true || analysis["has_audio"] != true || len(analysis["allowed_rate_permille"].([]any)) == 0 {
		t.Fatal("the source's sound and rate facts are incomplete", analysis)
	}
	segment := analysis["segments"].([]any)[0].(map[string]any)
	if segment["observation_id"] != clip.ObservationID("source", 0) {
		t.Fatal("observations carry no reference the cuts can cite", segment)
	}
	// Nothing about a section, an item's place or a sentence: this call writes
	// the flow, and the response carries no text at all.
	if strings.Contains(system, "template_section_id") || strings.Contains(system, "admitted_sections") || strings.Contains(system, "short_text") {
		t.Fatal("the retired section and copy contract survived in the flow prompt")
	}
}

func TestFlowContractStatesWhereARateComesFrom(t *testing.T) {
	_, _, system := flowRequest(t, flowInput(), defaultFlow())
	for _, sentence := range []string{
		"the activity, the camera motion, the scene type and whether speech is audible", // CLIP-127
		"stays at 1000",                        // an unsupported span
		"At most 40 % of the cuts",             // CLIP-128
		"no minimum asks you to transform any", // CLIP-128
		"audible speech stays at 1000 while its source has retains_original_audio true", // CLIP-129
		"Never choose a rate to reach the target duration",                              // CLIP-99
		"the order of the footage, its rhythm and how many cuts there are follow it",    // CLIP-136
		"the cut budget follows target_duration_ms and the footage that was actually observed",
	} {
		if !strings.Contains(system, sentence) {
			t.Fatal("the flow contract does not state: " + sentence)
		}
	}
}

func TestFlowPlansAnOverShareAndASpeechRateAsWritten(t *testing.T) {
	// Both cuts leave 1x and the first one holds audible speech on a source
	// whose sound the owner keeps: the share and the speech rule bind the
	// WRITING, and a delivered plan above them renders as written (CLIP-128).
	plan, _, _ := flowRequest(t, flowInput(), flowResponse(flowCut("cut-one", 0, 9375, 1250), flowCut("cut-two", 15000, 24375, 1250)))
	if len(plan.Cuts) != 2 || plan.Cuts[0].Rate() != 1250 || plan.Cuts[1].Rate() != 1250 {
		t.Fatal("a transformed cut was returned to 1x, moving every millisecond after it", plan.Cuts)
	}
	for _, notice := range clip.ActivePlanNotices(plan, composition.DefaultDesign()) {
		if notice.Reason == "plan_cut_rate" || notice.Reason == "plan_cut_usability" {
			t.Fatal("the writing bound was enforced at render", notice)
		}
	}
}

func TestFlowResolvesTheFixedRegionsAndWritesNoNarration(t *testing.T) {
	plan, _, _ := flowRequest(t, flowInput(), defaultFlow())
	roles := map[string]int{}
	for _, text := range plan.Portable.Elements {
		roles[text.Resolved.Element.Role]++
		if text.Scope == clip.NarrationScope {
			t.Fatal("the flow call wrote a caption", text.Resolved)
		}
		if text.Resolved.CutID != "" {
			t.Fatal("a region was bound to a cut", text.Resolved)
		}
	}
	if roles["badge"] != 1 || roles["hook"] != 1 || roles["ending"] != 1 || len(plan.Portable.Elements) != 3 {
		t.Fatal("the template's fixed regions did not resolve", roles)
	}
	for _, cut := range plan.Portable.Cuts {
		if cut.SectionID != "" || cut.GroupID != "" || cut.ItemID != "" {
			t.Fatal("a cut claimed a place in a template that declares none", cut)
		}
	}
	// A plan written today has no section and no item, so the notices that
	// reported their order and binding can never be recorded again.
	for _, reason := range []string{"composition_section_order", "composition_item_order", "item_unassigned", "copy_not_generated"} {
		if hasNotice(plan, reason) || hasFallback(plan, "cut-one", reason) {
			t.Fatal("a retired notice was recorded: " + reason)
		}
	}
	if _, err := clip.EncodeEditPlan(plan); err != nil {
		t.Fatal("the flow's own plan is not storable", err)
	}
}

func TestFlowKeepsEverySelectionCheckTheWriterAnsweredBefore(t *testing.T) {
	for name, c := range map[string]struct {
		response string
		notice   string
		cuts     int
	}{
		"an unknown source":                {flowResponse(flowCut("cut-one", 0, 7500, 1000), map[string]any{"id": "cut-two", "source_id": "other", "start_ms": 7500, "end_ms": 15000, "rate_permille": 1000, "focal": map[string]any{"x": .5, "y": .5}, "volume": 1, "observation_refs": []string{}}), "plan_source", 1},
		"a repeated identity":              {flowResponse(flowCut("cut-one", 0, 7500, 1000), flowCut("cut-one", 7500, 15000, 1000)), "plan_cut_identity", 1},
		"footage another cut already used": {flowResponse(flowCut("cut-one", 0, 7500, 1000), flowCut("cut-two", 7500, 15000, 1000), flowCut("cut-three", 7000, 14000, 1000)), "plan_source_overlap", 2},
		"an identity nobody minted":        {flowResponse(flowCut("cut-one", 0, 7500, 1000), flowCut("두 번째", 7500, 15000, 1000)), "composition_cut_identity", 1},
	} {
		s, _, _ := newService(t, c.response, true)
		plan, _, err := s.Flow(t.Context(), testRef(), flowInput())
		if err != nil {
			t.Fatal(name, err)
		}
		if len(plan.Cuts) != c.cuts || !hasNotice(plan, c.notice) {
			t.Fatal(name+": the offending cut was kept or unexplained", plan.Cuts, clip.ActivePlanNotices(plan, composition.DefaultDesign()))
		}
	}
	// A cut that crosses an observed boundary is narrowed into ONE scene, the
	// repair the single writer's cuts already received (CLIP-7).
	crossing, _, _ := flowRequest(t, flowInput(), flowResponse(flowCut("cut-one", 10000, 20000, 1000)))
	if len(crossing.Cuts) != 1 || crossing.Cuts[0].StartMS < 0 || crossing.Cuts[0].EndMS > 15000 {
		t.Fatal("a cut was delivered across two observed scenes", crossing.Cuts)
	}
	// Unusable footage is never selected automatically, whoever asked for it.
	unusable := flowInput()
	unusable.Analyses[0].Segments[1].Usability = clip.UsabilityUnusable
	refused, _, _ := flowRequest(t, unusable, defaultFlow())
	if len(refused.Cuts) != 2 || !hasNotice(refused, "plan_cut_usability") {
		t.Fatal("unusable footage was selected", refused.Cuts, clip.ActivePlanNotices(refused, composition.DefaultDesign()))
	}
	for _, cut := range refused.Cuts {
		if cut.StartMS >= 15000 {
			t.Fatal("a cut was taken from the unusable scene", cut)
		}
	}
}

func TestTheSingleWriterNoLongerWritesAComposition(t *testing.T) {
	s, models, _ := newService(t, defaultFlow(), true)
	_, _, err := s.Plan(t.Context(), testRef(), flowInput())
	if err == nil || len(models.calls) != 0 {
		t.Fatal("a composition reached the retired single writer", err)
	}
}
