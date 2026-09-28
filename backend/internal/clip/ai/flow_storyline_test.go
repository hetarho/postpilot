package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
)

// storylineFlow is defaultFlow opening with the given storyline paragraphs.
func storylineFlow(paragraphs ...map[string]any) string {
	var fields map[string]any
	if err := json.Unmarshal([]byte(defaultFlow()), &fields); err != nil {
		panic(err)
	}
	values := []any{}
	for _, p := range paragraphs {
		values = append(values, p)
	}
	fields["storyline"] = values
	return raw(fields)
}

func paragraph(text string, ids ...string) map[string]any {
	if ids == nil {
		ids = []string{}
	}
	return map[string]any{"text": text, "observation_ids": ids}
}

// CLIP-178: on 바로 만들기 the flow answer opens with the storyline, kept on the plan in order,
// naming only observed scenes — an unknown id is dropped and a repeated one stays where it was
// first named — while the flow itself is admitted exactly as before.
func TestTheFlowKeepsTheStorylineItOpensWith(t *testing.T) {
	scene := clip.ObservationID("source", 0)
	plan, _, system := flowRequest(t, flowInput(), storylineFlow(
		paragraph("가게 앞에서 시작해요.", scene, "source/99"),
		paragraph("음식을 담는 장면으로 이어져요.", scene),
		paragraph("   "),
	))
	if plan.Storyline == nil || len(plan.Storyline.Paragraphs) != 2 {
		t.Fatalf("the storyline was not kept: %+v", plan.Storyline)
	}
	first, second := plan.Storyline.Paragraphs[0], plan.Storyline.Paragraphs[1]
	if first.Text != "가게 앞에서 시작해요." || len(first.ObservationIDs) != 1 || first.ObservationIDs[0] != scene {
		t.Fatalf("the first paragraph lost its scene or kept an unknown one: %+v", first)
	}
	if second.Text != "음식을 담는 장면으로 이어져요." || len(second.ObservationIDs) != 0 {
		t.Fatalf("a repeated scene was kept outside its first paragraph: %+v", second)
	}
	if plan.Storyline.EditedByHand || len(plan.Storyline.MadeWithSources) != 0 {
		t.Fatal("the parse decided what only the save knows", plan.Storyline)
	}
	if len(plan.Cuts) != 3 {
		t.Fatal("the storyline changed the flow", plan.Cuts)
	}
	for _, sentence := range []string{"its storyline, then ordered cuts", "Before the cuts, set storyline", "Then choose the cuts along it", "beyond the storyline"} {
		if !strings.Contains(system, sentence) {
			t.Fatal("the flow contract does not state: " + sentence)
		}
	}
	// An empty storyline is none; the flow stands.
	if plan, _, _ := flowRequest(t, flowInput(), defaultFlow()); plan.Storyline != nil || len(plan.Cuts) != 3 {
		t.Fatal("an empty storyline was kept or cost the flow", plan.Storyline)
	}
}

// The bounds: thirty paragraphs, a thousand characters each, and the byte cap that keeps the
// narration request inside its allowance (CLIP-90).
func TestTheStorylineIsHeldToItsBounds(t *testing.T) {
	long := strings.Repeat("가", clip.StorylineTextMaxChars+50)
	paragraphs := []map[string]any{paragraph(long)}
	for range clip.StorylineParagraphMax + 5 {
		paragraphs = append(paragraphs, paragraph("장면"))
	}
	plan, _, _ := flowRequest(t, flowInput(), storylineFlow(paragraphs...))
	if plan.Storyline == nil {
		t.Fatal("the storyline was lost")
	}
	if len(plan.Storyline.Paragraphs) != clip.StorylineParagraphMax {
		t.Fatalf("%d paragraphs kept, want %d", len(plan.Storyline.Paragraphs), clip.StorylineParagraphMax)
	}
	if got := []rune(plan.Storyline.Paragraphs[0].Text); len(got) != clip.StorylineTextMaxChars {
		t.Fatalf("a text of %d characters was kept", len(got))
	}
	// Twelve paragraphs of a thousand syllables pass the byte cap; the rest are dropped.
	many := []map[string]any{}
	for range 12 {
		many = append(many, paragraph(strings.Repeat("가", clip.StorylineTextMaxChars)))
	}
	plan, _, _ = flowRequest(t, flowInput(), storylineFlow(many...))
	total := 0
	for _, p := range plan.Storyline.Paragraphs {
		total += len(p.Text)
	}
	if total > clip.StorylineMaxBytes || len(plan.Storyline.Paragraphs) != clip.StorylineMaxBytes/(3*clip.StorylineTextMaxChars) {
		t.Fatalf("the byte cap kept %d bytes in %d paragraphs", total, len(plan.Storyline.Paragraphs))
	}
}

// CLIP-131: a revision's flow rewrite answers the contract without a storyline, never writes
// one, and a response that sends one anyway is outside that contract.
func TestARevisionsFlowRewriteWritesNoStoryline(t *testing.T) {
	in := revisionInput(t, clip.RevisionFlow, "더 빠르게")
	plan, _, systems := revise(t, in, defaultFlow(), narrationResponse())
	if plan.Storyline != nil {
		t.Fatal("a revision wrote a storyline", plan.Storyline)
	}
	if strings.Contains(systems[0], "set storyline") || !strings.Contains(systems[0], "this response carries no text") {
		t.Fatal("the revision's flow call was asked for a storyline")
	}
	if strings.Contains(string(ai.RevisionFlowSchema()), "storyline") || strings.Contains(string(ai.RevisionFlowSchema()), "region_slots") || !strings.Contains(string(ai.FlowSchema()), "storyline") || !strings.Contains(string(ai.FlowSchema()), "region_slots") {
		t.Fatal("the two flow schemas are not the direct one and the storyline-free one")
	}
	s, models := newService(t, storylineFlow(paragraph("다시 쓴 이야기")), true)
	models.responses = []string{storylineFlow(paragraph("다시 쓴 이야기"))}
	if _, _, err := s.Revise(t.Context(), testRef(), in); err == nil {
		t.Fatal("a revision's flow answer carrying a storyline was accepted")
	}
}

// CLIP-178: the narration writes along the storyline the flow opened with — it is in the
// payload, paragraphs with their scenes, and the contract says to follow it; with none, the
// request is the one it always was.
func TestTheNarrationWritesAlongTheStoryline(t *testing.T) {
	in := narrationInput(t)
	scene := clip.ObservationID("source", 0)
	in.Flow.Storyline = &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "가게 앞", ObservationIDs: []string{scene}}, {Text: "마무리"}}}
	_, payload, system := narrate(t, in, narrationResponse(narrationCaption("가게 앞이에요", 1000, 5000)))
	storyline, ok := payload["storyline"].([]any)
	if !ok || len(storyline) != 2 {
		t.Fatal("the narration was not given the storyline", payload["storyline"])
	}
	first := storyline[0].(map[string]any)
	if first["text"] != "가게 앞" || len(first["observation_ids"].([]any)) != 1 {
		t.Fatal("a paragraph lost its text or its scenes", first)
	}
	if !strings.Contains(system, "Write the captions along storyline, part by part, in its order.") {
		t.Fatal("the narration contract does not say to follow the storyline")
	}
	_, plain, plainSystem := narrate(t, narrationInput(t), narrationResponse(narrationCaption("가게 앞이에요", 1000, 5000)))
	if _, present := plain["storyline"]; present || strings.Contains(plainSystem, "along storyline") {
		t.Fatal("a narration with no storyline was sent one")
	}
}

// CLIP-90: the allowance the start measures the narration against carries the widest storyline
// the flow may open with.
func TestThePreparationMeasuresTheWidestStoryline(t *testing.T) {
	flow := ai.WidestFlow(ai.DefaultConfig(clip.Environment{}), flowInput())
	if flow.Storyline == nil || len(flow.Storyline.Paragraphs) != clip.StorylineParagraphMax {
		t.Fatal("the widest flow carries no widest storyline", flow.Storyline)
	}
	total := 0
	for _, p := range flow.Storyline.Paragraphs {
		total += len(p.Text)
	}
	if total > clip.StorylineMaxBytes || total < clip.StorylineMaxBytes-3*clip.StorylineParagraphMax {
		t.Fatalf("the widest storyline measures %d bytes against a cap of %d", total, clip.StorylineMaxBytes)
	}
}
