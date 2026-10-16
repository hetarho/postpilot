package ai_test

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
)

var guidelines = clip.VideoGuidelines{
	Defaults: []string{"입력한 사실만 쓰기", "감상은 내가 쓴 것만", "연결한 글의 구체적인 표현을 자막에 살리기"},
	Owner:    []string{"자막에 가격을 적지 않기", "첫 컷은 가게 외관으로"},
}

const guidelineBlock = "\n[영상 지침]\n기본 지침:\n- 입력한 사실만 쓰기\n- 감상은 내가 쓴 것만\n- 연결한 글의 구체적인 표현을 자막에 살리기\n사용자 지침:\n- 자막에 가격을 적지 않기\n- 첫 컷은 가게 외관으로\n" +
	"영상 지침은 이 영상의 흐름과 자막을 어떻게 만들지 정합니다. 사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고, 같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요.\n"

// GUIDE-15, GUIDE-17: every clip writing call's system prompt ENDS with the frozen 영상 지침 —
// the 기본 지침 group, then the owner's, each in the order frozen, then the precedence sentence.
func TestEveryClipWritingCallEndsWithTheVideoGuidelines(t *testing.T) {
	post := &clip.LinkedPostText{Slug: "owned", Title: "내 경험", Sections: []clip.LinkedPostSection{{Text: "내가 남긴 구체적인 평가"}}}
	in := flowInput()
	in.Guidelines = guidelines
	in.LinkedPost = post
	_, _, flow := flowRequest(t, in, defaultFlow())
	if !strings.HasSuffix(flow, guidelineBlock) {
		t.Fatalf("the flow prompt does not end with the block:\n%s", flow[max(0, len(flow)-400):])
	}

	narration := narrationInput(t)
	narration.Guidelines = guidelines
	narration.LinkedPost = post
	_, _, system := narrate(t, narration, narrationResponse(narrationCaption("고기를 올렸어요", 1000, 5000)))
	if !strings.HasSuffix(system, guidelineBlock) {
		t.Fatalf("the narration prompt does not end with the block:\n%s", system[max(0, len(system)-400):])
	}

	if !strings.Contains(flow, "linked_post is the owner's own finished account") {
		t.Fatal("linked account framing missing beside video rules")
	}

	// A revision appends its own block first, so the 영상 지침 stay last.
	revision := revisionInput(t, clip.RevisionFlow, "더 빠르게")
	revision.Guidelines = guidelines
	revision.LinkedPost = post
	_, _, systems := revise(t, revision, defaultFlow(), narrationResponse())
	if len(systems) != 2 {
		t.Fatal("a flow revision makes two writing calls", len(systems))
	}
	for i, system := range systems {
		if !strings.HasSuffix(system, guidelineBlock) || !strings.Contains(system, "This is a REVISION") {
			t.Fatalf("revision call %d does not end with the block after its own:\n%s", i, system[max(0, len(system)-400):])
		}
	}
}

// GUIDE-52, GUIDE-54: a frozen 원칙 renders first under its own label, before the 기본 지침 and
// the owner's lines, and the 원칙 sentence precedes the closing sentence.
func TestVideoPrinciplesLeadTheSection(t *testing.T) {
	flowScope := []clip.VideoRuleApplicability{{Stage: "clip-write", Outputs: []string{"plan", "placements", "captions"}}}
	in := flowInput()
	in.Guidelines = clip.VideoGuidelines{
		Stock: []clip.VideoStockRule{
			{Key: "clip_facts", Text: "입력한 사실만", Applicability: flowScope, Principle: true},
			{Key: "clip_hook", Text: "첫 자막에 무엇을", SourceOrder: 1, Applicability: flowScope},
		},
		Owner: []string{"자막은 두 줄까지"},
	}
	_, _, system := flowRequest(t, in, defaultFlow())
	want := "\n[영상 지침]\n원칙:\n- 입력한 사실만\n기본 지침:\n- 첫 자막에 무엇을\n사용자 지침:\n- 자막은 두 줄까지\n" +
		"원칙은 끌 수 없으며 기본 지침, 사용자 지침, 템플릿 지시와 광고주 가이드보다 우선합니다.\n" +
		"영상 지침은 이 영상의 흐름과 자막을 어떻게 만들지 정합니다. 사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고, 같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요.\n"
	if !strings.HasSuffix(system, want) {
		t.Fatalf("the flow prompt does not end with the tiered block:\n%s", system[max(0, len(system)-500):])
	}
}

// One group alone renders alone, and the precedence sentence still closes the block.
func TestTheVideoGuidelinesRenderOnlyTheGroupsThatExist(t *testing.T) {
	in := flowInput()
	in.Guidelines = clip.VideoGuidelines{Owner: []string{"자막은 두 줄까지"}}
	_, _, system := flowRequest(t, in, defaultFlow())
	if !strings.HasSuffix(system, "\n[영상 지침]\n사용자 지침:\n- 자막은 두 줄까지\n영상 지침은 이 영상의 흐름과 자막을 어떻게 만들지 정합니다. 사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고, 같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요.\n") {
		t.Fatalf("an owner-only block rendered wrong:\n%s", system[max(0, len(system)-300):])
	}
	if strings.Contains(system, "기본 지침:") {
		t.Fatal("an empty 기본 지침 group was rendered")
	}
}

// No 영상 지침 adds no bytes at all: the prefix stays the one the goldens pin, for the flow, the
// narration and a revision alike.
func TestNoVideoGuidelinesAddNoBytes(t *testing.T) {
	_, _, flow := flowRequest(t, flowInput(), defaultFlow())
	_, _, narration := narrate(t, narrationInput(t), narrationResponse(narrationCaption("고기를 올렸어요", 1000, 5000)))
	_, _, revisions := revise(t, revisionInput(t, clip.RevisionNarration, "짧게"), narrationResponse())
	for _, system := range append([]string{flow, narration}, revisions...) {
		if strings.Contains(system, "[영상 지침]") || strings.Contains(system, "영상 지침은") {
			t.Fatal("a clip with no 영상 지침 was sent the block")
		}
	}
}

// CLIP-90: the allowance the start measures is the request the call sends, block included.
func TestThePreparationAllowanceCountsTheVideoGuidelines(t *testing.T) {
	in := flowInput()
	system, _ := ai.BuildFlowPrompt(in, 300, clip.DefaultCompositionLimits())
	in.Guidelines = guidelines
	withBlock, _ := ai.BuildFlowPrompt(in, 300, clip.DefaultCompositionLimits())
	if withBlock != system+guidelineBlock {
		t.Fatal("the measured flow request is not the one sent with the block")
	}
}
