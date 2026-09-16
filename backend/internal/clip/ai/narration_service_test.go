package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// narrationInput is the flow fixture already written and resolved: three cuts
// of 7.5 s, so the narration writes over a 22.5 s output timeline.
func narrationInput(t *testing.T) clip.NarrationInput {
	t.Helper()
	in := flowInput()
	s, _, _ := newService(t, defaultFlow(), true)
	flow, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	return clip.NarrationInput{PlanningInput: in, Flow: flow}
}

func narrationCaption(text string, start, end int, facts ...map[string]any) map[string]any {
	refs := []any{}
	for _, f := range facts {
		refs = append(refs, f)
	}
	return map[string]any{"id": "model-" + text, "text": text, "short_text": "", "keyword": "",
		"start_ms": start, "end_ms": end, "observation_refs": []string{clip.ObservationID("source", 0)}, "fact_refs": refs}
}
func narrationResponse(captions ...map[string]any) string {
	values := []any{}
	for _, c := range captions {
		values = append(values, c)
	}
	return raw(map[string]any{"captions": values, "slots": []any{}})
}

func narrate(t *testing.T, in clip.NarrationInput, response string) (clip.EditPlan, map[string]any, string) {
	t.Helper()
	s, models, _ := newService(t, response, true)
	plan, _, err := s.Narrate(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatal("the narration is one writing call", len(models.calls))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(models.calls[0].Messages[0].Parts[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	return plan, payload, models.calls[0].System
}

func narrationOf(plan clip.EditPlan) []clip.PortableText {
	var out []clip.PortableText
	for _, text := range plan.Portable.Elements {
		if text.Scope == clip.NarrationScope {
			out = append(out, text)
		}
	}
	return out
}
func hasReason(plan clip.EditPlan, reason string) bool {
	for _, f := range plan.Portable.Fallbacks {
		if f.Reason == reason {
			return true
		}
	}
	return hasNotice(plan, reason)
}

func TestNarrationRequestCarriesTheResolvedFlowAndNothingToChangeIt(t *testing.T) {
	in := narrationInput(t)
	in.Instruction = "고기 이야기를 해줘"
	_, payload, system := narrate(t, in, narrationResponse(narrationCaption("고기를 올렸어요", 1000, 4000)))
	for _, key := range []string{"project_instruction", "template_guide", "global_values", "item_groups", "item_hints", "analyses", "generated_region_slots", "cuts", "output_duration_ms"} {
		if _, ok := payload[key]; !ok {
			t.Fatal("the narration request lost " + key)
		}
	}
	if payload["output_duration_ms"].(float64) != float64(in.Flow.DurationMS) {
		t.Fatal("the writer was told a different output length", payload["output_duration_ms"])
	}
	cuts := payload["cuts"].([]any)
	if len(cuts) != len(in.Flow.Cuts) {
		t.Fatal("the flow did not reach the narration", cuts)
	}
	second := cuts[1].(map[string]any)
	if second["output_start_ms"].(float64) != 7500 || second["output_end_ms"].(float64) != 15000 || second["rate_permille"].(float64) != 1000 {
		t.Fatal("a cut's window on the output timeline is wrong", second)
	}
	if len(second["observation_ids"].([]any)) == 0 || second["start_ms"].(float64) != 7500 {
		t.Fatal("a cut lost its source range or its observations", second)
	}
	for _, sentence := range []string{
		"The flow is FINAL",
		"a cuts key in your response is ignored",
		"ABSOLUTE integer times on the output timeline",
		"Captions never overlap one another",
		"At most 100 captions",
		"at most 2 lines of 11 characters each",
		"a rapid phrase is at most 14",
		"at least 900 + 90 × characters ms",
		"may play over footage it does not describe",
		"A taste, texture, satisfaction or visit claim may be written only when project_instruction asks for it",
	} {
		if !strings.Contains(system, sentence) {
			t.Fatal("the narration contract does not state: " + sentence)
		}
	}
}

func TestNarrationCaptionsHoldDisjointAbsoluteWindows(t *testing.T) {
	in := narrationInput(t)
	plan, _, _ := narrate(t, in, narrationResponse(
		narrationCaption("첫 장면입니다", 1000, 5000),
		// Out of order on the wire and overlapping the first: the server orders
		// by start and refuses the collision rather than moving either caption.
		narrationCaption("세 번째 장면", 16000, 20000),
		narrationCaption("겹치는 자막", 4000, 8000),
	))
	captions := narrationOf(plan)
	if len(captions) != 2 {
		t.Fatal("an overlapping caption was admitted", captions)
	}
	if captions[0].Resolved.StartMS != 1000 || captions[0].Resolved.EndMS != 5000 || captions[1].Resolved.StartMS != 16000 {
		t.Fatal("a caption was retimed", captions[0].Resolved, captions[1].Resolved)
	}
	// The identities are the server's, minted in start order.
	if captions[0].Resolved.InstanceID != "narration-1" || captions[1].Resolved.InstanceID != "narration-2" {
		t.Fatal("the writer's own caption ids reached the plan", captions[0].Resolved.InstanceID, captions[1].Resolved.InstanceID)
	}
	if !hasReason(plan, clip.NoticeCaptionOverlap) {
		t.Fatal("the removed caption was not explained", plan.Portable.Fallbacks)
	}
	// The flow underneath is untouched.
	if len(plan.Cuts) != len(in.Flow.Cuts) || plan.DurationMS != in.Flow.DurationMS {
		t.Fatal("the narration changed the flow", plan.Cuts, plan.DurationMS)
	}
	for _, text := range captions {
		if text.Resolved.CutID != "" || text.Resolved.Element.Basis != "output-start" {
			t.Fatal("a caption was bound to a cut", text.Resolved)
		}
	}
	if _, err := clip.EncodeEditPlan(plan); err != nil {
		t.Fatal("the narrated plan is not storable", err)
	}
}

func TestNarrationIntervalOutsideTheOutputIsRemoved(t *testing.T) {
	in := narrationInput(t)
	plan, _, _ := narrate(t, in, narrationResponse(
		narrationCaption("좋은 자막", 1000, 5000),
		narrationCaption("끝 너머 자막", 22000, 30000),
	))
	if len(narrationOf(plan)) != 1 || !hasReason(plan, clip.NoticeCaptionOutsideOutput) {
		t.Fatal("a caption beyond the output was kept or unexplained", narrationOf(plan), plan.Portable.Fallbacks)
	}
}

func TestNarrationFloorTakesTheShorterSentenceThenTheRoomThenNothing(t *testing.T) {
	in := narrationInput(t)
	long := "아주 긴 문장을 하나 씁니다"
	floor := clip.MinExposureMS(long)
	// The window is too short for the sentence, and the room after it is free:
	// the caption keeps its start and takes the time it needs to be read.
	plan, _, _ := narrate(t, in, narrationResponse(narrationCaption(long, 1000, 1500)))
	captions := narrationOf(plan)
	if len(captions) != 1 || captions[0].Resolved.StartMS != 1000 || captions[0].Resolved.EndMS != 1000+floor {
		t.Fatal("the caption did not take the free room it needed", captions)
	}
	// With the next caption starting immediately there is no room: the shorter
	// sentence is used instead.
	short := map[string]any{"id": "m", "text": long, "short_text": "짧은 문장", "keyword": "",
		"start_ms": 1000, "end_ms": 1000 + clip.MinExposureMS("짧은 문장"), "observation_refs": []string{clip.ObservationID("source", 0)}, "fact_refs": []any{}}
	plan, _, _ = narrate(t, in, narrationResponse(short, narrationCaption("다음 자막", 1000+clip.MinExposureMS("짧은 문장"), 12000)))
	captions = narrationOf(plan)
	if len(captions) != 2 || captions[0].Resolved.Text != "짧은 문장" || captions[0].FallbackReason == "" {
		t.Fatal("the grounded shorter sentence was not used", captions)
	}
	// No shorter sentence and no room at all: the caption is removed and said so.
	crowded := map[string]any{"id": "m", "text": long, "short_text": "", "keyword": "",
		"start_ms": 1000, "end_ms": 1300, "observation_refs": []string{clip.ObservationID("source", 0)}, "fact_refs": []any{}}
	plan, _, _ = narrate(t, in, narrationResponse(crowded, narrationCaption("다음 자막", 1300, 6000)))
	if len(narrationOf(plan)) != 1 || !hasReason(plan, clip.NoticeCaptionFloor) {
		t.Fatal("an unreadable caption was kept or unexplained", narrationOf(plan), plan.Portable.Fallbacks)
	}
}

func TestNarrationGroundsNumbersAndNeedsAnInstructionForAnExperience(t *testing.T) {
	in := narrationInput(t)
	fact := map[string]any{"field_id": "price", "group_id": "menu", "item_id": "sea"}
	// A number some collected fact states, cited, and naming the item it belongs
	// to even though another item's footage is on screen.
	plan, _, _ := narrate(t, in, narrationResponse(narrationCaption("해물라면 12,000원", 1000, 6000, fact)))
	captions := narrationOf(plan)
	if len(captions) != 1 || len(captions[0].Resolved.Facts) != 1 || captions[0].Resolved.Facts[0].ItemID != "sea" {
		t.Fatal("a grounded caption lost its cited fact", captions)
	}
	if len(captions[0].Evidence) == 0 {
		t.Fatal("a caption kept no observation", captions[0])
	}
	// A number no fact states is removed with its reason.
	plan, _, _ = narrate(t, in, narrationResponse(narrationCaption("해물라면 9,000원", 1000, 6000, fact)))
	if len(narrationOf(plan)) != 0 || !hasReason(plan, "unsupported_number_unit") {
		t.Fatal("an ungrounded number survived", narrationOf(plan), plan.Portable.Fallbacks)
	}
	// An experiential claim needs an instruction that asked for it.
	tasted := narrationResponse(narrationCaption("국물이 고소했어요", 1000, 6000))
	plan, _, _ = narrate(t, in, tasted)
	if len(narrationOf(plan)) != 0 || !hasReason(plan, "unsupported_experience") {
		t.Fatal("an unasked experience survived", narrationOf(plan), plan.Portable.Fallbacks)
	}
	instructed := in
	instructed.Instruction = "먹어본 맛을 이야기해줘"
	plan, _, _ = narrate(t, instructed, tasted)
	if len(narrationOf(plan)) != 1 {
		t.Fatal("the instruction did not admit the experience it asked for", plan.Portable.Fallbacks)
	}
}

func TestNarrationBoundsRepetitionAndCharacterCount(t *testing.T) {
	in := narrationInput(t)
	overlong := strings.Repeat("긴", design.Caption().Lines*design.Caption().Chars+1)
	plan, _, _ := narrate(t, in, narrationResponse(narrationCaption(overlong, 1000, 12000)))
	if len(narrationOf(plan)) != 0 || !hasReason(plan, "composition_generated_bounds") {
		t.Fatal("a caption over its character bound survived", narrationOf(plan), plan.Portable.Fallbacks)
	}
	plan, _, _ = narrate(t, in, narrationResponse(
		narrationCaption("같은 문장", 1000, 5000),
		narrationCaption("같은 문장", 6000, 10000),
	))
	if len(narrationOf(plan)) != 1 || !hasReason(plan, "repeated_copy") {
		t.Fatal("the same sentence was said twice", narrationOf(plan), plan.Portable.Fallbacks)
	}
}

func TestNarrationRecordsNothingForWhatTheWriterSimplyDidNotSay(t *testing.T) {
	in := narrationInput(t)
	// One caption over a 22.5 s clip: every other moment is silent, two items
	// are never named and a whole cut carries nothing. None of that is an event.
	plan, _, _ := narrate(t, in, narrationResponse(narrationCaption("고기를 올렸어요", 1000, 5000)))
	for _, reason := range []string{"copy_not_generated", "item_unassigned", "item_uncertain", "cross_item_identity", "context_item_claim", "composition_section_order"} {
		if hasReason(plan, reason) {
			t.Fatal("the writer's own choice was recorded as a defect: " + reason)
		}
	}
	if len(plan.Portable.Fallbacks) != 0 {
		t.Fatal("a silent moment recorded a notice", plan.Portable.Fallbacks)
	}
}

func TestNarrationIgnoresCutsAndUnknownSlotIdentities(t *testing.T) {
	in := narrationInput(t)
	response := raw(map[string]any{
		"captions": []any{narrationCaption("고기를 올렸어요", 1000, 5000)},
		"slots":    []any{map[string]any{"element_id": "nobody", "rows": []string{"x"}, "short_rows": []string{}, "observation_refs": []string{}, "fact_refs": []any{}}},
		"cuts":     []any{map[string]any{"id": "cut-one", "start_ms": 0, "end_ms": 1000}},
	})
	plan, _, _ := narrate(t, in, response)
	if len(plan.Cuts) != len(in.Flow.Cuts) || plan.Cuts[0].EndMS != in.Flow.Cuts[0].EndMS {
		t.Fatal("the response changed the flow", plan.Cuts)
	}
	if !hasNotice(plan, "composition_generated_identity") {
		t.Fatal("the ignored identities were not recorded", clip.ActivePlanNotices(plan))
	}
	if len(narrationOf(plan)) != 1 {
		t.Fatal("the captions beside them were lost", narrationOf(plan))
	}
}

func TestNarrationWritesTheTemplatesOwnSlotRows(t *testing.T) {
	in := narrationInput(t)
	body := strings.Replace(flowBody,
		`<text id="hook" kind="fixed" role="hook" basis="output-start" start="0" end="2"><row><value field="place"/></row></text>`,
		`<text id="hook" kind="fixed" role="hook" basis="output-start" start="0" end="2"><row kind="ai">한 줄</row><row><value field="place"/></row></text>`, 1)
	in.Template.CompositionBody, in.Composition.Snapshot.Body = body, body
	in.Flow.Portable.Snapshot.Body = body
	s, _, _ := newService(t, defaultFlow(), true)
	flow, _, err := s.Flow(t.Context(), testRef(), in.PlanningInput)
	if err != nil {
		t.Fatal(err)
	}
	in.Flow = flow
	response := raw(map[string]any{
		"captions": []any{},
		"slots":    []any{map[string]any{"element_id": "hook", "rows": []string{"성수 곱창", ""}, "short_rows": []string{}, "observation_refs": []string{clip.ObservationID("source", 0)}, "fact_refs": []any{}}},
	})
	plan, payload, _ := narrate(t, in, response)
	if len(payload["generated_region_slots"].([]any)) != 1 {
		t.Fatal("the writer was not told which slot to write", payload["generated_region_slots"])
	}
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.ID != "hook" {
			continue
		}
		if len(text.Resolved.Rows) != 2 || text.Resolved.Rows[0].Text != "성수 곱창" || text.Resolved.Rows[1].Text != "성수 곱창" {
			t.Fatal("the written slot row or its fixed neighbour is wrong", text.Resolved.Rows)
		}
		return
	}
	t.Fatal("the region with a written row is missing", plan.Portable.Elements)
}

func TestNarrationRefusesAFlowItCannotWriteOver(t *testing.T) {
	in := narrationInput(t)
	s, models, _ := newService(t, narrationResponse(), true)
	empty := in
	empty.Flow = clip.EditPlan{}
	if _, _, err := s.Narrate(t.Context(), testRef(), empty); err == nil || len(models.calls) != 0 {
		t.Fatal("the narration ran without a resolved flow", err)
	}
	legacy := in
	legacy.Composition = nil
	if _, _, err := s.Narrate(t.Context(), testRef(), legacy); err == nil || len(models.calls) != 0 {
		t.Fatal("the narration ran without a composition", err)
	}
}
