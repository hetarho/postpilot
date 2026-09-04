package generation

import (
	"strings"
	"testing"
)

func slotBrief() *TemplateBrief {
	return &TemplateBrief{
		Name:  "리뷰",
		Body:  "<write>인트로</write>\n" + SlotTokenForTest(1) + "\n<write>본문</write>\n" + SlotTokenForTest(2),
		Slots: []TemplateSlot{{Kind: "place", Label: "네이버 지도"}, {Kind: "link", Label: "예약 링크"}},
	}
}

// SlotTokenForTest exposes the token form to this package's tests without widening the API.
func SlotTokenForTest(n int) string { return slotToken(n) }

func text(content string) Block { return Block{Type: BlockText, Content: content} }

func kinds(content PostContent) []string {
	out := make([]string, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		if block.Slot != nil {
			out = append(out, "slot:"+block.Slot.Kind)
			continue
		}
		out = append(out, string(block.Type)+":"+block.Content)
	}
	return out
}

func TestApplyTemplateSlotsTurnsTokensIntoSlotBlocksInPlace(t *testing.T) {
	content := PostContent{Blocks: []Block{
		text("인트로 문단"), text(slotToken(1)), text("본문 문단"), text(slotToken(2)),
	}}

	got := kinds(ApplyTemplateSlots(content, slotBrief()))
	want := []string{"TEXT:인트로 문단", "slot:place", "TEXT:본문 문단", "slot:link"}
	if len(got) != len(want) {
		t.Fatalf("blocks = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("blocks = %v, want %v", got, want)
		}
	}
	// The label the author wrote is what a person filling it sees.
	result := ApplyTemplateSlots(content, slotBrief())
	if result.Blocks[1].Slot.Label != "네이버 지도" || result.Blocks[3].Slot.Label != "예약 링크" {
		t.Fatalf("labels = %+v / %+v", result.Blocks[1].Slot, result.Blocks[3].Slot)
	}
}

// A10: a slot the model never echoed is inserted rather than lost — a post that looks
// finished and silently dropped the one thing a person still has to do is the failure here.
func TestApplyTemplateSlotsInsertsAnOmittedSlot(t *testing.T) {
	content := PostContent{Blocks: []Block{text("인트로"), text(slotToken(1)), text("본문")}}

	got := ApplyTemplateSlots(content, slotBrief())
	slots := 0
	for _, block := range got.Blocks {
		if block.Slot != nil {
			slots++
		}
	}
	if slots != 2 {
		t.Fatalf("slots = %d, want 2 (%v)", slots, kinds(got))
	}
	// The omitted one lands after the slot that did resolve, not at the very end.
	if got.Blocks[1].Slot == nil || got.Blocks[2].Slot == nil {
		t.Fatalf("the inserted slot did not follow its predecessor: %v", kinds(got))
	}
}

func TestApplyTemplateSlotsAppendsWhenNoTokenCameBack(t *testing.T) {
	content := PostContent{Blocks: []Block{text("인트로"), text("본문")}}

	got := ApplyTemplateSlots(content, slotBrief())
	if len(got.Blocks) != 4 || got.Blocks[2].Slot == nil || got.Blocks[3].Slot == nil {
		t.Fatalf("blocks = %v", kinds(got))
	}
}

// A token buried in a paragraph keeps the paragraph: the model wrote something real around it.
func TestApplyTemplateSlotsStripsATokenFromProseAndKeepsBoth(t *testing.T) {
	content := PostContent{Blocks: []Block{text("여기가 " + slotToken(1) + " 위치입니다"), text(slotToken(2))}}

	got := ApplyTemplateSlots(content, slotBrief())
	if len(got.Blocks) != 3 {
		t.Fatalf("blocks = %v", kinds(got))
	}
	if got.Blocks[0].Slot == nil || got.Blocks[1].Content != "여기가  위치입니다" {
		t.Fatalf("blocks = %v", kinds(got))
	}
}

func TestApplyTemplateSlotsDropsARepeatedToken(t *testing.T) {
	content := PostContent{Blocks: []Block{text(slotToken(1)), text(slotToken(1)), text(slotToken(2))}}

	got := ApplyTemplateSlots(content, slotBrief())
	slots := 0
	for _, block := range got.Blocks {
		if block.Slot != nil {
			slots++
		}
	}
	if slots != 2 {
		t.Fatalf("a repeated token duplicated a slot: %v", kinds(got))
	}
}

// A9: the pass is idempotent, which is what lets a revision run it again over content that
// already carries slot blocks — the position is kept rather than re-appended.
func TestApplyTemplateSlotsIsIdempotent(t *testing.T) {
	once := ApplyTemplateSlots(PostContent{Blocks: []Block{
		text("인트로"), text(slotToken(1)), text("본문"), text(slotToken(2)),
	}}, slotBrief())
	twice := ApplyTemplateSlots(once, slotBrief())

	first, second := kinds(once), kinds(twice)
	if len(first) != len(second) {
		t.Fatalf("second pass changed the block list: %v then %v", first, second)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("second pass changed the block list: %v then %v", first, second)
		}
	}
}

// A post with no template, or a template with no slots, is untouched byte for byte.
func TestApplyTemplateSlotsIsANoOpWithoutSlots(t *testing.T) {
	content := PostContent{Blocks: []Block{text("인트로"), text("본문")}}
	for name, brief := range map[string]*TemplateBrief{
		"no template": nil,
		"no slots":    {Name: "x", Body: "<write>a</write>"},
	} {
		got := ApplyTemplateSlots(content, brief)
		if len(got.Blocks) != 2 || got.Blocks[0].Content != "인트로" || got.Blocks[1].Content != "본문" {
			t.Fatalf("%s: blocks = %v", name, kinds(got))
		}
	}
}

// The model echoed only the LATER token. The omitted earlier slot must land before it, not
// after: template order is the only position information this pass has, and inserting relative
// to "the last slot seen" reversed them.
func TestApplyTemplateSlotsKeepsTemplateOrderWhenOnlyALaterTokenCameBack(t *testing.T) {
	content := PostContent{Blocks: []Block{text("인트로"), text(slotToken(2)), text("본문")}}

	got := ApplyTemplateSlots(content, slotBrief())
	want := []string{"TEXT:인트로", "slot:place", "slot:link", "TEXT:본문"}
	if diff := kinds(got); len(diff) != len(want) {
		t.Fatalf("blocks = %v, want %v", diff, want)
	}
	for i, block := range kinds(got) {
		if block != want[i] {
			t.Fatalf("blocks = %v, want %v", kinds(got), want)
		}
	}
}

// Two different tokens crammed into one paragraph: BOTH slots resolve and neither token is
// left in the prose, so a second pass changes nothing.
func TestApplyTemplateSlotsResolvesEveryTokenInOneBlock(t *testing.T) {
	content := PostContent{Blocks: []Block{text(slotToken(1) + " 그리고 " + slotToken(2))}}

	got := ApplyTemplateSlots(content, slotBrief())
	for _, block := range got.Blocks {
		if strings.Contains(block.Content, "{{slot:") && block.Slot == nil {
			t.Fatalf("a raw token survived in the prose: %v", kinds(got))
		}
	}
	slots := 0
	for _, block := range got.Blocks {
		if block.Slot != nil {
			slots++
		}
	}
	if slots != 2 {
		t.Fatalf("slots = %d, want 2 (%v)", slots, kinds(got))
	}
	// The prose between them survives as its own block.
	if len(got.Blocks) != 3 || got.Blocks[2].Content != "그리고" {
		t.Fatalf("blocks = %v", kinds(got))
	}
	// And the pass is idempotent over that result.
	again := kinds(ApplyTemplateSlots(got, slotBrief()))
	first := kinds(got)
	for i := range first {
		if first[i] != again[i] {
			t.Fatalf("second pass changed the block list: %v then %v", first, again)
		}
	}
}
