package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// revisionInput is a saved plan — the flow written and the narration over it —
// with an owner's request about it.
func revisionInput(t *testing.T, target, request string) clip.RevisionInput {
	t.Helper()
	in := narrationInput(t)
	s, _, _ := newService(t, narrationResponse(narrationCaption("고기를 올렸어요", 1000, 5000)), true)
	current, _, err := s.Narrate(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	return clip.RevisionInput{PlanningInput: in.PlanningInput, Current: current, Request: request, Target: target}
}

func revise(t *testing.T, in clip.RevisionInput, responses ...string) (clip.EditPlan, []map[string]any, []string) {
	t.Helper()
	s, models, _ := newService(t, responses[0], true)
	models.responses = responses
	plan, _, err := s.Revise(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	payloads := []map[string]any{}
	systems := []string{}
	for _, call := range models.calls {
		var payload map[string]any
		// A revision appends its own object after the request's; the writer
		// reads both, and the test reads the appended one.
		body := call.Messages[0].Parts[0].Text
		if err := json.Unmarshal([]byte(body[strings.LastIndex(body[:len(body)-2], "\n{"):]), &payload); err != nil {
			t.Fatal(err, body)
		}
		payloads = append(payloads, payload)
		systems = append(systems, call.System)
	}
	return plan, payloads, systems
}

func TestAFlowRevisionRewritesTheFootageAndThenWhatIsSaidOverIt(t *testing.T) {
	in := revisionInput(t, clip.RevisionFlow, "고기 장면을 먼저 보여줘")
	plan, payloads, systems := revise(t, in,
		flowResponse(flowCut("cut-one", 15000, 22500, 1000), flowCut("cut-two", 0, 7500, 1000)),
		narrationResponse(narrationCaption("다시 쓴 자막", 1000, 5000)),
	)
	if len(payloads) != 2 {
		t.Fatal("a flow revision did not make both writing calls", len(payloads))
	}
	// Both calls carry the plan as the owner's edits left it and the request.
	for i, payload := range payloads {
		current, ok := payload["current_plan"].(map[string]any)
		if !ok || payload["revision_request"] != in.Request {
			t.Fatal("call", i, "lost the request or the current plan", payload)
		}
		if len(current["cuts"].([]any)) != len(in.Current.Cuts) {
			t.Fatal("call", i, "did not carry the current flow", current["cuts"])
		}
		if len(current["captions"].([]any)) != 1 {
			t.Fatal("call", i, "did not carry the current narration", current["captions"])
		}
		if !strings.Contains(systems[i], "This is a REVISION of the plan in current_plan") {
			t.Fatal("call", i, "was not told it is a revision")
		}
	}
	// The flow call is not told the flow is final; the narration is written
	// over the flow this revision just wrote.
	if strings.Contains(systems[0], "FINAL for this revision") {
		t.Fatal("the flow revision was told not to change the flow")
	}
	if plan.Cuts[0].StartMS != 15000 || len(plan.Cuts) != 2 {
		t.Fatal("the rewritten flow was not the one returned", plan.Cuts)
	}
	if len(narrationOf(plan)) != 1 || narrationOf(plan)[0].Resolved.Text != "다시 쓴 자막" {
		t.Fatal("the narration was not rewritten over it", narrationOf(plan))
	}
}

func TestANarrationRevisionRewritesOnlyWhatIsSaid(t *testing.T) {
	in := revisionInput(t, clip.RevisionNarration, "가격을 말해줘")
	plan, payloads, systems := revise(t, in, narrationResponse(narrationCaption("해물라면 12,000원", 1000, 6000, map[string]any{"field_id": "price", "group_id": "menu", "item_id": "sea"})))
	if len(payloads) != 1 {
		t.Fatal("a narration revision made more than one writing call", len(payloads))
	}
	if !strings.Contains(systems[0], "FINAL for this revision") {
		t.Fatal("the narration revision was not told the flow stays")
	}
	// The cuts are exactly the ones the owner had.
	if len(plan.Cuts) != len(in.Current.Cuts) {
		t.Fatal("a narration revision changed the flow", plan.Cuts)
	}
	for i, cut := range plan.Cuts {
		if cut.ID != in.Current.Cuts[i].ID || cut.StartMS != in.Current.Cuts[i].StartMS || cut.EndMS != in.Current.Cuts[i].EndMS {
			t.Fatal("a cut moved under a narration revision", cut, in.Current.Cuts[i])
		}
	}
	if len(narrationOf(plan)) != 1 || narrationOf(plan)[0].Resolved.Text != "해물라면 12,000원" {
		t.Fatal("the narration was not the rewritten one", narrationOf(plan))
	}
}

func TestARevisionAnswersOnTheSameContractsAsAGeneration(t *testing.T) {
	// An ungrounded number is removed here exactly as it is in a generation.
	in := revisionInput(t, clip.RevisionNarration, "가격을 말해줘")
	plan, _, _ := revise(t, in, narrationResponse(narrationCaption("해물라면 9,000원", 1000, 6000)))
	if len(narrationOf(plan)) != 0 || !hasReason(plan, "unsupported_number_unit") {
		t.Fatal("a revision admitted what a generation refuses", narrationOf(plan), plan.Portable.Fallbacks)
	}
	// An over-share flow is planned as written, the writing bound being the
	// writer's to keep (CLIP-128).
	share := revisionInput(t, clip.RevisionFlow, "더 빠르게")
	fast, _, _ := revise(t, share,
		flowResponse(flowCut("cut-one", 0, 9375, 1250), flowCut("cut-two", 15000, 24375, 1250)),
		narrationResponse(),
	)
	if fast.Cuts[0].Rate() != 1250 || fast.Cuts[1].Rate() != 1250 {
		t.Fatal("a revision re-rated a delivered cut", fast.Cuts)
	}
}

func TestARevisionRefusesWhatItCannotBeAbout(t *testing.T) {
	s, models, _ := newService(t, narrationResponse(), true)
	base := revisionInput(t, clip.RevisionNarration, "가격을 말해줘")
	for name, broken := range map[string]func(clip.RevisionInput) clip.RevisionInput{
		"no request at all":        func(in clip.RevisionInput) clip.RevisionInput { in.Request = ""; return in },
		"a request past its bound": func(in clip.RevisionInput) clip.RevisionInput { in.Request = strings.Repeat("가", 1001); return in },
		"a target nobody declared": func(in clip.RevisionInput) clip.RevisionInput { in.Target = "everything"; return in },
		"no plan to revise": func(in clip.RevisionInput) clip.RevisionInput {
			in.Current = clip.EditPlan{}
			return in
		},
		"a payload with no composition": func(in clip.RevisionInput) clip.RevisionInput {
			in.Composition = nil
			return in
		},
	} {
		if _, _, err := s.Revise(t.Context(), testRef(), broken(base)); err == nil {
			t.Fatal("a revision ran with " + name)
		}
	}
	if len(models.calls) != 0 {
		t.Fatal("a refused revision still paid for a call", len(models.calls))
	}
}
