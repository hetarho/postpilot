package ai_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// A template whose outline carries two caption entries: one the template wrote
// in full, one it left to the writer (CLIP-112).
const declaredCaptionBody = `<clip version="1" accent="teal">
<field id="place" label="상호" required="true">가게 이름</field>
<group id="menu" label="메뉴" min="1"><field id="name" label="메뉴" required="true">메뉴 이름</field><field id="price" label="가격">가격</field></group>
<text id="badge" kind="fixed" role="badge" position="header" basis="whole">직접 작성</text>
<text id="hook" kind="fixed" role="hook" basis="output-start" start="0" end="2"><row><value field="place"/></row></text>
<text id="opening_line" kind="fixed" role="caption" basis="whole"><value field="place"/> 다녀왔어요</text>
<text id="dish_line" kind="ai" role="caption" basis="whole">첫 음식이 나오는 순간을 한 줄로 쓰세요</text>
<text id="ending" kind="fixed" role="ending" basis="output-end" start="-2" end="0"><row>또 갈래요</row></text></clip>`

func declaredNarrationInput(t *testing.T) clip.NarrationInput {
	t.Helper()
	in := flowInput()
	setNativeBody(&in, declaredCaptionBody)
	s, _, _ := newService(t, defaultFlow(), true)
	flow, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	return clip.NarrationInput{PlanningInput: in, Flow: flow}
}

func declaredCaption(id, text string, start, end int) map[string]any {
	return map[string]any{"element_id": id, "text": text, "short_text": "", "keyword": "",
		"start_ms": start, "end_ms": end, "observation_refs": []string{clip.ObservationID("source", 0)}, "fact_refs": []any{}}
}
func declaredResponse(declared []map[string]any, captions ...map[string]any) string {
	values, entries := []any{}, []any{}
	for _, c := range captions {
		values = append(values, c)
	}
	for _, d := range declared {
		entries = append(entries, d)
	}
	return raw(map[string]any{"captions": values, "slots": []any{}, "declared_captions": entries})
}

// A declared caption is a caption like any other once it is placed, so it is
// found by the words it carries rather than by an identity of its own.
func narrationText(plan clip.EditPlan, text string) *clip.PortableText {
	for i := range plan.Portable.Elements {
		if plan.Portable.Elements[i].Resolved.Text == text {
			return &plan.Portable.Elements[i]
		}
	}
	return nil
}

// CLIP-112: the writer places the outline's caption entries; CLIP-65: what a
// fixed one says is the template's, whatever the response returns for it.
func TestDeclaredCaptionsArePlacedByTheWriterAndKeepTheirAuthoredText(t *testing.T) {
	plan, payload, _ := narrate(t, declaredNarrationInput(t), declaredResponse(
		[]map[string]any{
			declaredCaption("opening_line", "모델이 멋대로 고쳐 쓴 문장", 0, 3000),
			declaredCaption("dish_line", "면을 그릇에 담고 있어요", 4000, 8000),
		},
		narrationCaption("김이 오르는 국물이에요", 9000, 13000),
	))
	entries, ok := payload["declared_captions"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatal("the request did not carry the outline's caption entries", payload["declared_captions"])
	}
	first, _ := json.Marshal(entries[0])
	second, _ := json.Marshal(entries[1])
	if !slices.Contains([]string{string(first)}, `{"element_id":"opening_line","kind":"fixed","order":1,"text":"성수 곱창 다녀왔어요"}`) {
		t.Fatal("the fixed entry did not reach the writer as written", string(first))
	}
	if !slices.Contains([]string{string(second)}, `{"element_id":"dish_line","instruction":"첫 음식이 나오는 순간을 한 줄로 쓰세요","kind":"ai","order":2}`) {
		t.Fatal("the ai entry did not reach the writer as an instruction", string(second))
	}
	fixed := narrationText(plan, "성수 곱창 다녀왔어요")
	if fixed == nil || !fixed.Authored {
		t.Fatal("the template's own words were replaced by the response", fixed)
	}
	if fixed.Resolved.StartMS != 0 || fixed.Resolved.EndMS != 3000 {
		t.Fatal("the writer's placement was not used", fixed.Resolved)
	}
	written := narrationText(plan, "면을 그릇에 담고 있어요")
	if written == nil || written.Authored {
		t.Fatal("the ai entry did not take the text the writer wrote", written)
	}
	// Every caption still holds its own window, declared or written (CDS-62).
	captions := narrationOf(plan)
	if len(captions) != 3 {
		t.Fatal("the declared entries did not join the narration", len(captions))
	}
	slices.SortFunc(captions, func(a, b clip.PortableText) int { return a.Resolved.StartMS - b.Resolved.StartMS })
	for i := 1; i < len(captions); i++ {
		if captions[i].Resolved.StartMS < captions[i-1].Resolved.EndMS {
			t.Fatal("two captions claimed the same moment", captions[i-1].Resolved, captions[i].Resolved)
		}
	}
}

// CLIP-108: an entry the response leaves out is shown nowhere and says so once,
// refusing nothing.
func TestADeclaredCaptionTheResponseOmitsIsNoticed(t *testing.T) {
	plan, _, _ := narrate(t, declaredNarrationInput(t), declaredResponse(
		[]map[string]any{declaredCaption("dish_line", "면을 그릇에 담고 있어요", 4000, 8000)},
		narrationCaption("김이 오르는 국물이에요", 9000, 13000),
	))
	if narrationText(plan, "성수 곱창 다녀왔어요") != nil {
		t.Fatal("an unplaced entry was shown anyway")
	}
	notices := 0
	for _, n := range clip.ActivePlanNotices(plan, plan.Design().RegionPresets()) {
		if n.ElementID == "opening_line" && n.Reason == "copy_omitted" && n.Action == "removal" {
			notices++
		}
	}
	if notices != 1 {
		t.Fatal("the omitted entry did not record exactly one notice", clip.ActivePlanNotices(plan, plan.Design().RegionPresets()))
	}
	if narrationText(plan, "면을 그릇에 담고 있어요") == nil {
		t.Fatal("the entry the writer did place was dropped with it")
	}
}

// An ai entry is the writer's own claim and answers to CLIP-137 like any other
// caption; the authored one beside it does not (CLIP-122).
func TestADeclaredAICaptionIsGroundedLikeAnyWrittenOne(t *testing.T) {
	plan, _, _ := narrate(t, declaredNarrationInput(t), declaredResponse(
		[]map[string]any{
			declaredCaption("opening_line", "", 0, 3000),
			declaredCaption("dish_line", "한 그릇에 99,000원이에요", 4000, 8000),
		},
	))
	if narrationText(plan, "한 그릇에 99,000원이에요") != nil {
		t.Fatal("an ungrounded number was admitted from a declared entry")
	}
	if !hasReason(plan, "unsupported_number_unit") {
		t.Fatal("the removal did not carry its reason", plan.Portable.Fallbacks)
	}
	if fixed := narrationText(plan, "성수 곱창 다녀왔어요"); fixed == nil {
		t.Fatal("the authored entry was ground checked with it", fixed)
	}
}

// The outline's order is guidance the writer follows where the footage allows,
// not a check: a response that places the second entry first is delivered as
// written (CLIP-141's rule, applied to the entries beside the stages).
func TestTheOutlineOrderOfDeclaredCaptionsIsNotChecked(t *testing.T) {
	plan, _, _ := narrate(t, declaredNarrationInput(t), declaredResponse(
		[]map[string]any{
			declaredCaption("dish_line", "면을 그릇에 담고 있어요", 1000, 5000),
			declaredCaption("opening_line", "", 6000, 9000),
		},
	))
	written, fixed := narrationText(plan, "면을 그릇에 담고 있어요"), narrationText(plan, "성수 곱창 다녀왔어요")
	if written == nil || fixed == nil {
		t.Fatal("a reversed placement dropped an entry", written, fixed)
	}
	if written.Resolved.StartMS >= fixed.Resolved.StartMS {
		t.Fatal("the response's own order was not delivered", written.Resolved, fixed.Resolved)
	}
	if hasReason(plan, "copy_omitted") {
		t.Fatal("the reversed order was recorded as a removal", plan.Portable.Fallbacks)
	}
}
