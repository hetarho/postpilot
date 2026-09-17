package ai_test

import (
	"strings"
	"testing"
)

// A template's named stages reach the flow call in the template guide's own
// position, as a numbered order to follow where the footage allows (CLIP-141).
// The block is pinned here because it is the only thing the writer reads them
// through: nothing in the server admits, refuses or reports a stage.
func TestTemplateStagesReachTheFlowAsNumberedGuidance(t *testing.T) {
	in := flowInput()
	body := strings.Replace(flowBody, "<guide>음식을 차분하게 보여준다.</guide>",
		"<guide>음식을 차분하게 보여준다.</guide>\n<stage name=\"가게 도착\">간판과\n외관을 먼저 보여준다</stage>\n<stage name=\"음식\">주문한 메뉴가 나오는 순간</stage>", 1)
	in.Template.CompositionBody = body
	in.Composition.Snapshot.Body = body
	_, payload, _ := flowRequest(t, in, defaultFlow())
	guide, _ := payload["template_guide"].(string)
	want := "음식을 차분하게 보여준다.\n" +
		"Composition stages, in this order, to follow where the footage allows. They admit and forbid no footage: skip or merge a stage nothing was filmed for, and keep footage matching no stage where it belongs. Never repeat or stretch footage to fill a stage.\n" +
		"1. 가게 도착 — 간판과 외관을 먼저 보여준다\n" +
		"2. 음식 — 주문한 메뉴가 나오는 순간"
	if guide != want {
		t.Fatalf("stage block\n%s\nwant\n%s", guide, want)
	}
}

// A template carrying no stage sends the guide it always sent, byte for byte,
// and one carrying nothing else still sends no empty section.
func TestATemplateWithoutStagesIsUnchanged(t *testing.T) {
	_, payload, _ := flowRequest(t, flowInput(), defaultFlow())
	if guide, _ := payload["template_guide"].(string); guide != "음식을 차분하게 보여준다." {
		t.Fatalf("the guide changed for a template with no stage: %q", guide)
	}
	silent := flowInput()
	body := strings.Replace(flowBody, "<guide>음식을 차분하게 보여준다.</guide>",
		"<stage name=\"가게 도착\">간판을 먼저 보여준다</stage>", 1)
	silent.Template.CompositionBody = body
	silent.Composition.Snapshot.Body = body
	_, stagesOnly, _ := flowRequest(t, silent, defaultFlow())
	guide, _ := stagesOnly["template_guide"].(string)
	if !strings.HasPrefix(guide, "Composition stages,") || !strings.HasSuffix(guide, "1. 가게 도착 — 간판을 먼저 보여준다") {
		t.Fatalf("stages alone reach the call as their own block: %q", guide)
	}
}
