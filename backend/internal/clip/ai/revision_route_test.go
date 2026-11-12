package ai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
)

// routeAnswer is an auto revision's first answer: the narration revision when the flow
// stays, or the revised flow when it does not (CLIP-131, CLIP-132).
func routeAnswer(flowChanged bool, document string) string {
	var doc map[string]any
	if err := json.Unmarshal([]byte(revisionFlow(document)), &doc); err != nil {
		panic(err)
	}
	branch := "narration"
	if flowChanged {
		branch = "flow"
	}
	return raw(map[string]any{"flow_changed": flowChanged, branch: doc})
}

func TestAnAutoRevisionThatKeepsTheFlowFinishesAfterOneCall(t *testing.T) {
	in := revisionInput(t, clip.RevisionAuto, "가격을 말해줘")
	var routed []string
	in.Routed = func(stage string) { routed = append(routed, stage) }
	s, models := newService(t, routeAnswer(false, narrationResponse(narrationCaption("해물라면 12,000원", 1000, 6000))), true)
	plan, _, err := s.Revise(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatal("a narration-only auto revision made more than one writing call", len(models.calls))
	}
	if strings.Join(routed, ",") != "narrate" {
		t.Fatal("the routing result was not reported as narration", routed)
	}
	if len(plan.Cuts) != len(in.Current.Cuts) {
		t.Fatal("a kept flow changed", plan.Cuts)
	}
	for i, cut := range plan.Cuts {
		if cut.ID != in.Current.Cuts[i].ID || cut.StartMS != in.Current.Cuts[i].StartMS || cut.EndMS != in.Current.Cuts[i].EndMS {
			t.Fatal("a cut moved under a kept flow", cut, in.Current.Cuts[i])
		}
	}
	if len(narrationOf(plan)) != 1 || narrationOf(plan)[0].Resolved.Text != "해물라면 12,000원" {
		t.Fatal("the narration was not the routed one", narrationOf(plan))
	}
	call := models.calls[0]
	if !strings.Contains(call.System, "flow_changed") || !strings.Contains(call.System, "NARRATION contract") {
		t.Fatal("the routing call was not told both contracts and the envelope")
	}
	if !json.Valid(call.JSONSchema) || !strings.Contains(string(call.JSONSchema), "flow_changed") || string(call.JSONSchema) != string(ai.RevisionRouteSchema()) {
		t.Fatal("the routing call did not carry the routing schema", string(call.JSONSchema))
	}
}

func TestAnAutoRevisionThatChangesTheFlowWritesTheNarrationOverIt(t *testing.T) {
	in := revisionInput(t, clip.RevisionAuto, "고기 장면을 먼저 보여줘")
	var routed []string
	in.Routed = func(stage string) { routed = append(routed, stage) }
	flow := routeAnswer(true, flowResponse(flowCut("cut-one", 15000, 22500, 1000), flowCut("cut-two", 0, 7500, 1000)))
	s, models := newService(t, flow, true)
	models.responses = []string{flow, narrationResponse(narrationCaption("다시 쓴 자막", 1000, 5000))}
	plan, _, err := s.Revise(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 2 || strings.Join(routed, ",") != "flow" {
		t.Fatal("a flow-changing auto revision did not make the flow then narration calls", len(models.calls), routed)
	}
	if plan.Cuts[0].SourceID == in.Current.Cuts[0].SourceID && plan.Cuts[0].StartMS == in.Current.Cuts[0].StartMS {
		t.Fatal("the routed flow was not used", plan.Cuts)
	}
	// Call 2 is the narration the flow target writes today.
	if strings.Contains(models.calls[1].System, "flow_changed") || strings.Contains(models.calls[1].System, "FINAL for this revision") {
		t.Fatal("call 2 was not the flow target's narration call")
	}
	if len(narrationOf(plan)) != 1 || narrationOf(plan)[0].Resolved.Text != "다시 쓴 자막" {
		t.Fatal("the narration was not written over the routed flow", narrationOf(plan))
	}
}

func TestAMalformedRoutingAnswerTakesTheBoundedCorrection(t *testing.T) {
	in := revisionInput(t, clip.RevisionAuto, "가격을 말해줘")
	in.Policy.ResponseRetries = 3
	missingBranch := raw(map[string]any{"flow_changed": false})
	kept := routeAnswer(false, narrationResponse(narrationCaption("해물라면 12,000원", 1000, 6000)))
	s, models := newService(t, missingBranch, true)
	models.responses = []string{missingBranch, kept}
	if _, _, err := s.Revise(t.Context(), testRef(), in); err != nil {
		t.Fatal("a corrected routing answer was refused", err)
	}
	if len(models.calls) != 2 || !strings.Contains(models.calls[1].System, "failed validation") {
		t.Fatal("the unreadable routing answer was not corrected once", len(models.calls))
	}
	// Exhausted correction fails like any other unreadable answer, with no narration call after it.
	s, models = newService(t, missingBranch, true)
	models.responses = []string{missingBranch, missingBranch, missingBranch, missingBranch}
	if _, _, err := s.Revise(t.Context(), testRef(), in); err == nil {
		t.Fatal("an unreadable routing answer was accepted")
	}
	if len(models.calls) != 4 {
		t.Fatal("routing correction was not bounded to three more attempts", len(models.calls))
	}
}

func TestANarratedAutoRevisionRoutesTheSameWay(t *testing.T) {
	in := revisionInput(t, clip.RevisionAuto, "더 짧게 설명해요")
	draft, err := clip.NewSpokenDraft([]string{"고기를 올렸어요", "식사가 준비됐어요"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	in.Current.Narration = &draft.Narration
	in.Current.Narration.Enabled = true
	kept := raw(map[string]any{"flow_changed": false, "narration": map[string]any{"spoken_lines": []string{"고기를 올렸어요", "준비됐어요"}}})
	s, models := newService(t, kept, true)
	got, _, err := s.Revise(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || got.Narration.Segments[1].Text != "준비됐어요" || got.Narration.Segments[1].Speech != nil {
		t.Fatal("a narrated narration-only auto revision was not one call", len(models.calls), got.Narration)
	}
	changed := routeAnswer(true, flowResponse(flowCut("cut-one", 15000, 22500, 1000), flowCut("cut-two", 0, 7500, 1000)))
	s, models = newService(t, changed, true)
	models.responses = []string{changed, `{"spoken_lines":["짧은 음성"]}`}
	got, _, err = s.Revise(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 2 || got.Narration.Segments[0].Text != "짧은 음성" || len(got.Portable.Elements) != len(in.Current.Portable.Elements) {
		t.Fatal("a narrated flow-changing auto revision did not write the script over the new flow", len(models.calls), got.Narration)
	}
}

func TestASelectedSceneScopesTheRevision(t *testing.T) {
	in := revisionInput(t, clip.RevisionNarration, "이 장면만 바꿔줘")
	in.ScopeCut = 1
	_, payloads, systems := revise(t, in, narrationResponse(narrationCaption("바꾼 자막", 1000, 5000)))
	scope, ok := payloads[0]["revision_scope"].(map[string]any)
	if !ok || scope["cut"] != float64(1) || !strings.Contains(systems[0], "revision_scope") {
		t.Fatal("the selected scene was not stated", payloads[0], systems[0])
	}
	if captions, _ := scope["captions"].([]any); len(captions) != 1 {
		t.Fatal("the scene's current captions were not named", scope)
	}
	whole := revisionInput(t, clip.RevisionNarration, "전체를 바꿔줘")
	_, payloads, systems = revise(t, whole, narrationResponse(narrationCaption("바꾼 자막", 1000, 5000)))
	if _, scoped := payloads[0]["revision_scope"]; scoped || strings.Contains(systems[0], "revision_scope") {
		t.Fatal("a whole-clip request named a scene")
	}
	s, models := newService(t, narrationResponse(), true)
	for _, cut := range []int{-1, len(in.Current.Cuts) + 1} {
		in.ScopeCut = cut
		if _, _, err := s.Revise(t.Context(), testRef(), in); !errors.Is(err, clip.ErrInvalid) {
			t.Fatal("a scene outside the plan was accepted", cut, err)
		}
	}
	if len(models.calls) != 0 {
		t.Fatal("a refused scope still paid for a call")
	}
}
