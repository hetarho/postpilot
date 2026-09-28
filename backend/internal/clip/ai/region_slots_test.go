package ai_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// draftedRegions are an intro (preset A) with one slot to write from an owner's
// instruction and one the owner fixed, and an outro (preset E) whose first slot
// an answer binds and whose other two are to be written with no instruction.
func draftedRegions() *clip.ProjectRegions {
	return &clip.ProjectRegions{
		Intro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{{ID: "project-intro-1", Instruction: "동네 이름", InstructionEdited: true}, {ID: "project-intro-2", Text: "골목 저녁", OwnerFixed: true}}},
		Outro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{{ID: "project-outro-1", Text: "성수 곱창", Binding: []composition.Part{{Field: "place"}}}, {ID: "project-outro-2"}, {ID: "project-outro-3"}}},
	}
}

func regionsInput() clip.PlanningInput {
	in := twoScenes()
	in.Design = clip.ProjectDesign{IntroPreset: "a", OutroPreset: "e"}
	in.Regions = draftedRegions()
	return in
}

func storylineAnswer(slots ...map[string]any) string {
	values := []any{}
	for _, s := range slots {
		values = append(values, s)
	}
	return raw(map[string]any{"storyline": []any{paragraph("가게 앞", clip.ObservationID("source", 0))}, "region_slots": values})
}

func slotAnswer(id, text string) map[string]any {
	return map[string]any{"slot_id": id, "text": text, "short_text": ""}
}

// The storyline call drafts the generated slots of the enabled regions with the
// body, in its one call (CLIP-187): each slot the owner fixed or an answer
// binds is shown with its words and never written, each other one comes with
// its instruction — empty when the owner gave none — and the bounds its preset
// gives it (CDS-86), and no business label is suggested for an empty one.
func TestTheStorylineCallDraftsTheEnabledSlotsWithTheBody(t *testing.T) {
	in := regionsInput()
	s, models := newService(t, storylineAnswer(slotAnswer("project-intro-1", "성수동"), slotAnswer("project-outro-2", "다시 올게요"), slotAnswer("project-intro-2", "덮어쓰기"), slotAnswer("nobody", "누구")), true)
	got, _, err := s.Storyline(t.Context(), testRef(), clip.StorylineInput{PlanningInput: in})
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatal("the slots cost a call of their own", len(models.calls))
	}
	var payload struct {
		Slots []struct {
			ID           string `json:"slot_id"`
			Region       string `json:"region"`
			Order        int    `json:"order"`
			Role         string `json:"role"`
			Lines        int    `json:"lines"`
			MaxSyllables int    `json:"max_syllables"`
			Write        bool   `json:"write"`
			Instruction  string `json:"instruction"`
			Text         string `json:"text"`
		} `json:"intro_outro"`
	}
	if err := json.Unmarshal([]byte(models.calls[0].Messages[0].Parts[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Slots) != 5 {
		t.Fatal("the writer was not shown every active slot", payload.Slots)
	}
	for i, want := range []struct {
		kind, id, slot string
		index          int
		write          bool
		words          string
	}{{"intro", "a", "project-intro-1", 0, true, "동네 이름"}, {"intro", "a", "project-intro-2", 1, false, "골목 저녁"}, {"outro", "e", "project-outro-1", 0, false, "성수 곱창"}, {"outro", "e", "project-outro-2", 1, true, ""}, {"outro", "e", "project-outro-3", 2, true, ""}} {
		spec, width, _ := design.RegionSlotAt(want.kind, want.id, in.Ratio, want.index)
		got := payload.Slots[i]
		words := got.Text
		if got.Write {
			words = got.Instruction
		}
		if got.ID != want.slot || got.Region != want.kind || got.Order != want.index+1 || got.Write != want.write || words != want.words || got.Role != spec.Role || got.Lines != spec.MaxLines() || got.MaxSyllables != design.RegionSlotBudget(spec, width) {
			t.Fatalf("slot %d: %+v", i, got)
		}
	}
	system := models.calls[0].System
	for _, label := range []string{"restaurant", "store name", "가게", "상호"} {
		if strings.Contains(system, label) {
			t.Fatal("the prompt suggests a business label", label)
		}
	}
	if !strings.Contains(system, "with an empty instruction, write what that slot's role holds in this clip") || !strings.Contains(string(models.calls[0].JSONSchema), "region_slots") {
		t.Fatal("the storyline call does not ask for the slots", system)
	}
	want := []clip.RegionDraft{{SlotID: "project-intro-1", Text: "성수동"}, {SlotID: "project-outro-2", Text: "다시 올게요"}, {SlotID: "project-outro-3"}}
	if !reflect.DeepEqual(got.RegionDrafts, want) {
		t.Fatal("the drafts are not the generated slots' words alone", got.RegionDrafts)
	}
	// A region that is off is not written at all, and with both off the section is
	// not sent (CLIP-187).
	quiet := regionsInput()
	quiet.Regions.Intro.Enabled, quiet.Regions.Outro.Enabled = false, false
	s, models = newService(t, storylineAnswer(slotAnswer("project-intro-1", "꺼진 인트로")), true)
	off, _, err := s.Storyline(t.Context(), testRef(), clip.StorylineInput{PlanningInput: quiet})
	if err != nil || strings.Contains(models.calls[0].Messages[0].Parts[0].Text, "intro_outro") || off.RegionDrafts != nil {
		t.Fatal("a region that is off was offered or written", off.RegionDrafts, err)
	}
}

// A storyline request may rewrite the generated slots and is shown what they
// hold now, so the words it does not ask about stay (CLIP-181, CLIP-187).
func TestAStorylineRequestIsShownTheSlotWordsItMayKeep(t *testing.T) {
	in := regionsInput()
	in.Regions.Intro.Slots[0].Text = "성수동"
	current := clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "가게 앞", ObservationIDs: []string{clip.ObservationID("source", 0)}}}}
	s, models := newService(t, storylineAnswer(slotAnswer("project-intro-1", "연남동")), true)
	got, _, err := s.Storyline(t.Context(), testRef(), clip.StorylineInput{PlanningInput: in, Current: &current, Request: "동네를 바꿔줘"})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Slots []map[string]any `json:"intro_outro"`
	}
	if err := json.Unmarshal([]byte(models.calls[0].Messages[0].Parts[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Slots[0]["current_text"] != "성수동" || !strings.Contains(models.calls[0].System, "keep it wherever the request does not ask for other words") {
		t.Fatal("the request was not shown the slot's current words", payload.Slots[0])
	}
	if got.RegionDrafts[0] != (clip.RegionDraft{SlotID: "project-intro-1", Text: "연남동"}) {
		t.Fatal(got.RegionDrafts)
	}
}

// 바로 만들기 drafts the slots in its flow call, with the storyline, and the plan
// carries the drafts to the save; a build from a reviewed storyline writes no
// slot, its words being the reviewed ones (CLIP-135, CLIP-187).
func TestDirectGenerationDraftsTheSlotsInItsFlowCall(t *testing.T) {
	in := regionsInput()
	var fields map[string]any
	if err := json.Unmarshal([]byte(storylineFlow(paragraph("가게 앞", clip.ObservationID("source", 0)))), &fields); err != nil {
		t.Fatal(err)
	}
	fields["region_slots"] = []any{slotAnswer("project-intro-1", "성수동"), slotAnswer("project-outro-2", strings.Repeat("하나둘셋넷", 12))}
	plan, payload, system := flowRequest(t, in, raw(fields))
	if !hasKey(payload, "intro_outro") || !strings.Contains(system, "Answer every slot with write true in region_slots") {
		t.Fatal("the direct flow call was not asked for the slots")
	}
	want := []clip.RegionDraft{{SlotID: "project-intro-1", Text: "성수동"}, {SlotID: "project-outro-2", Notice: "outro_slot_omitted"}, {SlotID: "project-outro-3"}}
	if plan.Storyline == nil || !reflect.DeepEqual(plan.Storyline.RegionDrafts, want) {
		t.Fatal("the flow's drafts are not carried with its storyline", plan.Storyline)
	}
	followed := in
	followed.FollowStoryline = &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "가게 앞", ObservationIDs: []string{clip.ObservationID("source", 0)}}}}
	s, models := newService(t, revisionFlow(defaultFlow()), true)
	built, _, err := s.Flow(t.Context(), testRef(), followed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(models.calls[0].Messages[0].Parts[0].Text, "intro_outro") || strings.Contains(string(models.calls[0].JSONSchema), "region_slots") || built.Storyline != nil {
		t.Fatal("a build from the reviewed storyline was asked to write the slots")
	}
}
