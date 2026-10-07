package generation

import (
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

func duplicatePrefixRevisionFixture() (PostContent, post.OriginReview) {
	current := PostContent{Blocks: []Block{{Type: BlockText, Content: "맛있었다"}, {Type: BlockText, Content: "맛있었다 그리고 달콤한 향"}}}
	candidates := []post.OriginCandidate{
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "맛있었다", post.OriginOwnerInput, "memo"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(1)}, "맛있었다", post.OriginAIAdded, "plan"),
	}
	return current, ResolveWriteOriginCandidates(current, originSources(), candidates).Review
}

func TestRevisionStructuralBlockMapDoesNotInheritDeletedSharedPrefixOrigin(t *testing.T) {
	current, prior := duplicatePrefixRevisionFixture()
	after := PostContent{Blocks: []Block{current.Blocks[1]}}
	candidate := originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "맛있었다", post.OriginAIAdded, "plan")
	got := PreserveRevisionOrigins(current, &prior, after, originSources(), []post.OriginCandidate{candidate})
	if len(got.Spans) != 1 || *got.Spans[0].Field.BlockIndex != 0 || got.Spans[0].Category != post.OriginAIAdded || !slices.Equal(got.Spans[0].SourceRefs, []string{"plan"}) {
		t.Fatalf("deleted block owner proof retargeted to shifted AI block: %+v", got)
	}
}

func TestRevisionStructuralBlockMapPreservesInsertAndReorderWithOwnEvidence(t *testing.T) {
	current, prior := duplicatePrefixRevisionFixture()
	for _, after := range []PostContent{
		{Blocks: []Block{{Type: BlockText, Content: "new fact"}, current.Blocks[0], current.Blocks[1]}},
		{Blocks: []Block{current.Blocks[1], current.Blocks[0]}},
	} {
		var candidates []post.OriginCandidate
		for i, block := range after.Blocks {
			field := post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(i)}
			quote := "맛있었다"
			ref := "memo"
			if block.Content == "new fact" {
				quote, ref = "new fact", "edit"
			}
			candidates = append(candidates, originCandidate(field, quote, post.OriginOwnerInput, ref))
		}
		got := PreserveRevisionOrigins(current, &prior, after, originSources(), candidates)
		if len(got.Spans) != len(after.Blocks) {
			t.Fatalf("proven insert/reorder lost spans: %+v", got)
		}
		for _, span := range got.Spans {
			block := after.Blocks[*span.Field.BlockIndex]
			if block.Content == current.Blocks[1].Content && (span.Category != post.OriginAIAdded || span.SourceRefs[0] != "plan") {
				t.Fatal("reordered AI block inherited owner proof from former index")
			}
		}
	}
}

func TestRevisionStructuralBlockMapLeavesAmbiguousDuplicateAndShiftedEditUnconfirmed(t *testing.T) {
	current, prior := duplicatePrefixRevisionFixture()
	after := PostContent{Blocks: []Block{{Type: BlockText, Content: "맛있었다 그리고 새 향"}}}
	candidate := originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "맛있었다", post.OriginOwnerInput, "memo")
	if len(PreserveRevisionOrigins(current, &prior, after, originSources(), []post.OriginCandidate{candidate}).Spans) != 0 {
		t.Fatal("unproved shifted partial edit inherited or recolored old prefix")
	}
	current.Blocks[1] = current.Blocks[0]
	prior = ResolveWriteOriginCandidates(current, originSources(), []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "맛있었다", post.OriginOwnerInput, "memo"), originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(1)}, "맛있었다", post.OriginAIAdded, "plan")}).Review
	after.Blocks = []Block{current.Blocks[0]}
	if len(PreserveRevisionOrigins(current, &prior, after, originSources(), []post.OriginCandidate{candidate}).Spans) != 0 {
		t.Fatal("duplicate retained block chosen by array index")
	}
}

func TestRevisionStructuralTagAndListMapsRetainUniqueShiftedMeaning(t *testing.T) {
	for _, list := range []bool{false, true} {
		before := PostContent{Tags: []string{"맛", "맛있음🙂"}}
		after := PostContent{Tags: []string{before.Tags[1]}}
		fields := []post.OriginFieldLocator{{Kind: post.OriginFieldTag, TagIndex: originInt(0)}, {Kind: post.OriginFieldTag, TagIndex: originInt(1)}}
		newField := post.OriginFieldLocator{Kind: post.OriginFieldTag, TagIndex: originInt(0)}
		if list {
			before = PostContent{Blocks: []Block{{Type: BlockList, Items: []string{"맛", "맛있음🙂"}}}}
			after = PostContent{Blocks: []Block{{Type: BlockList, Items: []string{before.Blocks[0].Items[1]}}}}
			fields = []post.OriginFieldLocator{{Kind: post.OriginFieldBlockItem, BlockIndex: originInt(0), ItemIndex: originInt(0)}, {Kind: post.OriginFieldBlockItem, BlockIndex: originInt(0), ItemIndex: originInt(1)}}
			newField = post.OriginFieldLocator{Kind: post.OriginFieldBlockItem, BlockIndex: originInt(0), ItemIndex: originInt(0)}
		}
		prior := ResolveWriteOriginCandidates(before, originSources(), []post.OriginCandidate{originCandidate(fields[0], "맛", post.OriginOwnerInput, "memo"), originCandidate(fields[1], "맛", post.OriginAIAdded, "plan")}).Review
		got := PreserveRevisionOrigins(before, &prior, after, originSources(), []post.OriginCandidate{originCandidate(newField, "맛", post.OriginOwnerInput, "memo")})
		if len(got.Spans) != 1 || got.Spans[0].Category != post.OriginAIAdded || got.Spans[0].SourceRefs[0] != "plan" || !originLocatorEqual(got.Spans[0].Field, newField) {
			t.Fatalf("indexed tag/list proof retargeted: list=%t result=%+v", list, got)
		}
	}
}

func TestRevisionPartialFieldMappingUsesUniqueAnchoredChangedSlot(t *testing.T) {
	before := PostContent{Blocks: []Block{{Type: BlockHeading, Content: "heading", Level: 2}, {Type: BlockText, Content: "old sentence. AI tail🙂"}, {Type: BlockQuote, Content: "footer"}}}
	field := post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(1)}
	prior := ResolveWriteOriginCandidates(before, originSources(), []post.OriginCandidate{originCandidate(field, "AI tail🙂", post.OriginAIAdded, "plan")}).Review
	after := PostContent{Blocks: []Block{before.Blocks[0], {Type: BlockText, Content: "new fact. AI tail🙂"}, before.Blocks[2]}}
	got := PreserveRevisionOrigins(before, &prior, after, originSources(), []post.OriginCandidate{originCandidate(field, "new fact", post.OriginOwnerInput, "edit"), originCandidate(field, "AI tail🙂", post.OriginOwnerInput, "memo")})
	if len(got.Spans) != 2 {
		t.Fatalf("proven anchored scalar edit lost new fact/old tail: %+v", got)
	}
	for _, span := range got.Spans {
		if span.Quote == "AI tail🙂" && span.Category != post.OriginAIAdded {
			t.Fatal("anchored suffix recolored")
		}
	}
}
