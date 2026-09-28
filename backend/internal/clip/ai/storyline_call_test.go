package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
)

// twoScenes is flowInput, whose one source holds two observed scenes, source/0 and source/1.
func twoScenes() clip.PlanningInput { return flowInput() }

func sceneCut(id string, start, end int, scene string) map[string]any {
	return map[string]any{"id": id, "source_id": "source", "start_ms": start, "end_ms": end, "rate_permille": 1000,
		"focal": map[string]any{"x": .5, "y": .5}, "volume": 1, "observation_refs": []string{scene}}
}

// CLIP-178, CDS-37: built from a storyline, the flow call is shown only the scenes it holds —
// each under its original observation id — and the paragraphs, keeps the storyline's order and
// pace, answers the contract without a storyline, and a cut on any other scene is removed.
func TestTheFlowBuiltFromAStorylineUsesOnlyTheScenesItHolds(t *testing.T) {
	in := twoScenes()
	held := clip.ObservationID("source", 1)
	in.FollowStoryline = &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "뒤쪽 장면", ObservationIDs: []string{held}}}, MadeWithSources: []string{"source"}}
	half := in.Analyses[0].Segments[1].StartMS
	response := raw(map[string]any{"ratio": "vertical", "duration_ms": 15000, "cuts": []any{
		sceneCut("cut-early", 0, 3000, clip.ObservationID("source", 0)),
		sceneCut("cut-late", half, half+3000, held),
	}})
	s, models := newService(t, response, true)
	plan, _, err := s.Flow(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	call := models.calls[0]
	var payload map[string]any
	if err := json.Unmarshal([]byte(call.Messages[0].Parts[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	segments := payload["analyses"].([]any)[0].(map[string]any)["segments"].([]any)
	if len(segments) != 1 || segments[0].(map[string]any)["observation_id"] != held {
		t.Fatalf("the writer was shown scenes the storyline does not hold: %v", segments)
	}
	if storyline, ok := payload["storyline"].([]any); !ok || len(storyline) != 1 {
		t.Fatal("the writer was not given the storyline to build along", payload["storyline"])
	}
	if !strings.Contains(call.System, "Build the cuts along storyline") || !strings.Contains(call.System, "outrank project_instruction's") || strings.Contains(call.System, "set storyline") {
		t.Fatal("the flow contract does not say to build along the storyline")
	}
	if strings.Contains(string(call.JSONSchema), "storyline") {
		t.Fatal("a build from the storyline was asked for a storyline of its own")
	}
	if plan.Storyline != nil {
		t.Fatal("a build from the storyline wrote one")
	}
	if len(plan.Cuts) != 1 || plan.Cuts[0].ID != "cut-late" {
		t.Fatalf("a cut on a scene the storyline does not hold was kept: %+v", plan.Cuts)
	}
	removed := false
	for _, n := range plan.Notices {
		removed = removed || n.Reason == "storyline_scene" && n.CutID == "cut-early"
	}
	if !removed {
		t.Fatal("the removal did not say why", plan.Notices)
	}
}

// CLIP-177: the storyline call reads the same material the flow does, sets the storyline and
// nothing else, and ends with the 영상 지침; a storyline request adds the current storyline, the
// owner's words and the one sentence that says to keep what they do not touch (CLIP-181).
func TestTheStorylineCallAsksForTheStorylineAlone(t *testing.T) {
	in := clip.StorylineInput{PlanningInput: twoScenes()}
	in.Guidelines = clip.VideoGuidelines{Owner: []string{"자막에 가격을 적지 않기"}}
	system, user := ai.BuildStorylinePrompt(in, clip.DefaultCompositionLimits())
	for _, sentence := range []string{"Set the clip's storyline from the material below", "Do not choose cuts or write captions", "each named once in the whole storyline"} {
		if !strings.Contains(system, sentence) {
			t.Fatal("the storyline contract does not state: " + sentence)
		}
	}
	if !strings.HasSuffix(system, "먼저 적힌 지침을 따르세요.\n") || strings.Contains(system, "Rewrite current_storyline") {
		t.Fatal("the storyline call does not end with its 영상 지침, or was asked to rewrite one")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"project_instruction", "global_values", "item_groups", "item_hints", "analyses", "template_outline"} {
		if _, ok := payload[key]; !ok {
			t.Fatal("the storyline call lost " + key)
		}
	}
	if _, ok := payload["current_storyline"]; ok {
		t.Fatal("스토리라인 먼저 was given a storyline to rewrite")
	}
	in.Current = &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "앞"}}}
	in.Request = "가게 소개를 먼저"
	system, user = ai.BuildStorylinePrompt(in, clip.DefaultCompositionLimits())
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["request"] != "가게 소개를 먼저" || payload["current_storyline"] == nil || !strings.Contains(system, "keep the paragraphs and the scene choices the request does not touch") {
		t.Fatal("the storyline request lost its storyline, its words or its rule", payload)
	}
}

// The answer is the storyline, held to its bounds and to observed scenes; one that keeps
// nothing is bad output the correction retries, never an empty storyline.
func TestTheStorylineAnswerIsBoundedAndNeverEmpty(t *testing.T) {
	answer := raw(map[string]any{"storyline": []any{paragraph("가게 앞", clip.ObservationID("source", 0), "nowhere/9")}, "region_slots": []any{}})
	s, _ := newService(t, answer, true)
	got, _, err := s.Storyline(t.Context(), testRef(), clip.StorylineInput{PlanningInput: twoScenes()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Paragraphs) != 1 || len(got.Paragraphs[0].ObservationIDs) != 1 || got.Paragraphs[0].ObservationIDs[0] != clip.ObservationID("source", 0) {
		t.Fatalf("the storyline kept an unknown scene or lost its own: %+v", got)
	}
	empty, _ := newService(t, raw(map[string]any{"storyline": []any{}, "region_slots": []any{}}), true)
	if _, _, err := empty.Storyline(t.Context(), testRef(), clip.StorylineInput{PlanningInput: twoScenes()}); err == nil {
		t.Fatal("an empty storyline was accepted")
	}
	// A request needs the storyline it rewrites.
	if _, _, err := s.Storyline(t.Context(), testRef(), clip.StorylineInput{PlanningInput: twoScenes(), Request: "다시"}); err == nil {
		t.Fatal("a storyline request with no storyline was sent")
	}
}
