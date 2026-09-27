package ai_test

import (
	"strings"
	"testing"
)

const stageHeader = "Composition stages, in this order, to follow where the footage allows. They admit and forbid no footage: skip or merge a stage nothing was filmed for, and keep footage matching no stage where it belongs. Never repeat or stretch footage to fill a stage."

// A template's named stages reach the flow call as the template outline, a numbered
// order to follow where the footage allows (CLIP-141), each line naming the stage
// and what it is about (CLIP-185). The block is pinned here because it is the only
// thing the writer reads them through: nothing in the server admits, refuses or
// reports a stage.
func TestTemplateStagesReachTheFlowAsANumberedOutline(t *testing.T) {
	in := flowInput()
	body := strings.Replace(flowBody, "<stage name=\"음식\">주문한 메뉴가 나오는 순간</stage>",
		"<stage name=\"가게 도착\">간판과\n외관</stage>\n<stage name=\"음식\">주문한 메뉴가 나오는 순간</stage>", 1)
	in.Template.CompositionBody = body
	in.Composition.Snapshot.Body = body
	_, payload, _ := flowRequest(t, in, defaultFlow())
	outline, _ := payload["template_outline"].(string)
	want := stageHeader + "\n" +
		"1. 가게 도착 — 간판과 외관\n" +
		"2. 음식 — 주문한 메뉴가 나오는 순간"
	if outline != want {
		t.Fatalf("stage block\n%s\nwant\n%s", outline, want)
	}
	if _, present := payload["template_guide"]; present {
		t.Fatal("the retired template guide key was sent")
	}
}

// The narration call carries the same outline, so both writing calls read one form.
func TestTheNarrationCarriesTheSameOutline(t *testing.T) {
	_, flowPayload, _ := flowRequest(t, flowInput(), defaultFlow())
	if outline, _ := flowPayload["template_outline"].(string); outline != stageHeader+"\n1. 음식 — 주문한 메뉴가 나오는 순간" {
		t.Fatalf("the flow outline = %q", outline)
	}
}
