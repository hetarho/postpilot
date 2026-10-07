package post

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func alignmentTestIndex(value int) *int { return &value }
func alignmentTestCandidate(field OriginFieldLocator, quote string, category OriginCategory, refs ...string) OriginCandidate {
	return OriginCandidate{Field: field, Quote: quote, Category: category, SourceRefs: refs}
}
func alignmentTestSources() []OriginSource {
	return []OriginSource{{ID: "memo", Kind: OriginSourceMemo, Text: "owner facts", Available: true}, {ID: "plan", Kind: OriginSourceAIProposal, Text: "frozen AI proposal", Available: true}, {ID: "photo", Kind: OriginSourceVisualObservation, Text: "a visible cup", AttachmentFilename: "a.jpg", Available: true}}
}
func alignmentTestReview(content PostContent, candidates []OriginCandidate) *OriginReview {
	result := ResolveOriginCandidates(content, ContentOriginIdentity(content, 7), alignmentTestSources(), candidates)
	return &result.Review
}
func alignmentTestSave(before, after PostContent, prior *OriginReview, attached ...string) OriginResolution {
	return AlignManualOriginReview(before, after, prior, ContentOriginIdentity(before, 7), ContentOriginIdentity(after, 8), "manual-8", attached)
}

func TestManualOriginAlignmentNoopKeepsExactSourceCategoryAndConfirmation(t *testing.T) {
	before := PostContent{Title: "title", Tags: []string{"tag"}, Blocks: []Block{{Type: BlockText, Content: "달콤한 향🙂"}}}
	field := OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(field, "달콤한 향🙂", OriginAIAdded, "plan")})
	prior.Spans[0].ReviewState = OriginConfirmed
	beforeReview := cloneOriginSpan(prior.Spans[0])
	sources := slices.Clone(prior.Sources)
	got := alignmentTestSave(before, before, prior, "a.jpg")
	if len(got.Review.Spans) != 1 || !reflect.DeepEqual(got.Review.Spans[0], beforeReview) || !reflect.DeepEqual(got.Review.Sources, sources) || got.Review.Result != ContentOriginIdentity(before, 8) {
		t.Fatalf("no-op guessed authorship/source or lost review state: %+v", got)
	}
	if !reflect.DeepEqual(prior.Spans[0], beforeReview) || !reflect.DeepEqual(prior.Sources, sources) {
		t.Fatal("alignment mutated original review")
	}
	if len(alignmentTestSave(before, before, nil, "a.jpg").Review.Spans) != 0 {
		t.Fatal("legacy no-op inferred origin from Save")
	}
}

func TestManualOriginAlignmentPunctuationAndStyleNeverPromoteExistingAI(t *testing.T) {
	field := OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}
	before := PostContent{Blocks: []Block{{Type: BlockText, Content: "달콤한 향🙂."}}}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(field, "달콤한 향🙂", OriginAIAdded, "plan")})
	for _, text := range []string{"달콤한 향🙂!", "(달콤한 향🙂).", "달콤한 향🙂. "} {
		after := PostContent{Blocks: []Block{{Type: BlockText, Content: text}}}
		got := alignmentTestSave(before, after, prior)
		for _, span := range got.Review.Spans {
			if span.Category != OriginAIAdded {
				t.Fatal("punctuation became ownership of existing AI meaning")
			}
		}
		if len(got.Review.Spans) != 1 || got.Review.Spans[0].Quote != "달콤한 향🙂" {
			t.Fatalf("exact punctuation-surrounded phrase lost: %+v", got)
		}
	}
	after := PostContent{Blocks: []Block{{Type: BlockText, Content: "말투만 바뀐 새로운 표현"}}}
	if len(alignmentTestSave(before, after, prior).Review.Spans) != 0 {
		t.Fatal("style/replacement was guessed as manual owner fact")
	}
}

func TestManualOriginAlignmentClassifiesOnlyActualNewFieldBlockAndPureInsert(t *testing.T) {
	before := PostContent{Blocks: []Block{{Type: BlockText, Content: "달콤한 향🙂"}}}
	field := OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(field, "달콤한 향🙂", OriginAIAdded, "plan")})
	prior.Spans[0].ReviewState = OriginConfirmed
	for _, inserted := range []struct{ prefix, suffix string }{{"좋았고 ", ""}, {"", " 새 사실"}} {
		after := PostContent{Title: "new title", Tags: []string{"new tag"}, Blocks: []Block{{Type: BlockText, Content: inserted.prefix + before.Blocks[0].Content + inserted.suffix}, {Type: BlockQuote, Content: "명시적으로 더한 새 사실"}}}
		got := alignmentTestSave(before, after, prior)
		var ai *OriginSpan
		owner := 0
		for i := range got.Review.Spans {
			span := &got.Review.Spans[i]
			if span.Category == OriginAIAdded {
				ai = span
			} else {
				owner++
				if span.Category != OriginOwnerInput || len(span.SourceRefs) != 1 || !strings.HasPrefix(span.SourceRefs[0], "manual-8.") {
					t.Fatal("new input lacks actual owner-edit source")
				}
			}
		}
		if ai == nil || ai.Start != utf8.RuneCountInString(inserted.prefix) || ai.End-ai.Start != utf8.RuneCountInString(ai.Quote) || ai.ReviewState != OriginConfirmed || owner != 4 {
			t.Fatalf("mixed inserted/known AI meaning lost: %+v", got)
		}
		for _, source := range got.Review.Sources {
			if strings.HasPrefix(source.ID, "manual-8.") && (source.Kind != OriginSourceOwnerEdit || strings.Contains(source.Text, "달콤한 향🙂")) {
				t.Fatal("Save text mislabeled old AI meaning owner input")
			}
		}
	}
}

func TestManualOriginAlignmentReindexesUniqueBlocksTagsListItemsWithoutPrefixLeak(t *testing.T) {
	before := PostContent{Tags: []string{"맛", "맛있음🙂"}, Blocks: []Block{{Type: BlockText, Content: "맛있었다"}, {Type: BlockText, Content: "맛있었다 그리고 향"}, {Type: BlockList, Items: []string{"맛", "맛있음🙂"}}}}
	candidates := []OriginCandidate{alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}, "맛있었다", OriginOwnerInput, "memo"), alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(1)}, "맛있었다", OriginAIAdded, "plan"), alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldTag, TagIndex: alignmentTestIndex(1)}, "맛", OriginAIAdded, "plan"), alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockItem, BlockIndex: alignmentTestIndex(2), ItemIndex: alignmentTestIndex(1)}, "맛", OriginAIAdded, "plan")}
	prior := alignmentTestReview(before, candidates)
	after := PostContent{Tags: []string{before.Tags[1]}, Blocks: []Block{before.Blocks[1], {Type: BlockList, Items: []string{before.Blocks[2].Items[1]}}}}
	got := alignmentTestSave(before, after, prior)
	if len(got.Review.Spans) != 3 {
		t.Fatalf("unique removed/reindexed fields lost: %+v", got)
	}
	for _, span := range got.Review.Spans {
		if span.Category != OriginAIAdded || !slices.Equal(span.SourceRefs, []string{"plan"}) {
			t.Fatal("removed prefix field recolored surviving field")
		}
	}
	after = PostContent{Tags: []string{before.Tags[1], before.Tags[0]}, Blocks: []Block{before.Blocks[2], before.Blocks[1], before.Blocks[0]}}
	got = alignmentTestSave(before, after, prior)
	if len(got.Review.Spans) != 4 {
		t.Fatal("unique reorder lost known sources")
	}
	for _, span := range got.Review.Spans {
		if span.Category == OriginOwnerInput && (span.Field.BlockIndex == nil || *span.Field.BlockIndex != 2) {
			t.Fatal("reorder was paired by prior array index")
		}
	}
}

func TestManualOriginAlignmentDuplicateAndAmbiguousReplacementRemainUnconfirmed(t *testing.T) {
	before := PostContent{Blocks: []Block{{Type: BlockText, Content: "same"}, {Type: BlockText, Content: "same"}}}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}, "same", OriginOwnerInput, "memo"), alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(1)}, "same", OriginAIAdded, "plan")})
	after := PostContent{Blocks: []Block{before.Blocks[1]}}
	if len(alignmentTestSave(before, after, prior).Review.Spans) != 0 {
		t.Fatal("duplicate deletion chose original source by position")
	}
	before.Blocks = []Block{{Type: BlockText, Content: "same"}, {Type: BlockText, Content: "same and more"}}
	prior = alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}, "same", OriginOwnerInput, "memo")})
	after.Blocks = []Block{{Type: BlockText, Content: "same and changed"}}
	if len(alignmentTestSave(before, after, prior).Review.Spans) != 0 {
		t.Fatal("unproved shifted replacement became owner input")
	}
}

func TestManualOriginAlignmentPhotoMoveKeepsEvidenceUntilActualRemoval(t *testing.T) {
	before := PostContent{Blocks: []Block{{Type: BlockText, Content: "cup"}, {Type: BlockImage, File: "a.jpg", Alt: "cup", Caption: "caption"}}}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}, "cup", OriginPhotoInterpretation, "photo"), alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockAlt, BlockIndex: alignmentTestIndex(1)}, "cup", OriginPhotoInterpretation, "photo")})
	after := PostContent{Blocks: []Block{before.Blocks[1], before.Blocks[0]}}
	got := alignmentTestSave(before, after, prior, "a.jpg")
	if len(got.Review.Spans) != 2 || got.Review.Spans[0].Category != OriginPhotoInterpretation {
		t.Fatal("photo movement promoted or removed actual evidence")
	}
	withoutImage := PostContent{Blocks: []Block{before.Blocks[0]}}
	if len(alignmentTestSave(before, withoutImage, prior, "a.jpg").Review.Spans) != 1 {
		t.Fatal("removing a display block withdrew still-attached photo evidence")
	}
	removed := alignmentTestSave(before, withoutImage, prior)
	if len(removed.Review.Spans) != 0 {
		t.Fatal("deleted actual source remained proof")
	}
	for _, source := range removed.Review.Sources {
		if source.ID == "photo" && source.Available {
			t.Fatal("withdrawn source was retargeted or restored")
		}
	}
	if !prior.Sources[2].Available {
		t.Fatal("actual removal mutated frozen caller evidence")
	}
}

func TestManualOriginAlignmentRejectsStaleAndWrongSourceKindsWithoutCanonicalChanges(t *testing.T) {
	before := PostContent{Title: "AI phrase", Blocks: []Block{{Type: BlockText, Content: "body"}}}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldTitle}, "AI phrase", OriginOwnerInput, "plan")})
	copy := before
	if len(alignmentTestSave(before, before, prior).Review.Spans) != 0 {
		t.Fatal("AI plan ref became owner fact from valid quote syntax")
	}
	prior = alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldTitle}, "AI phrase", OriginAIAdded, "plan")})
	prior.Result.ContentRevision++
	if len(alignmentTestSave(before, before, prior).Review.Spans) != 0 {
		t.Fatal("stale result was restored from current text")
	}
	if !reflect.DeepEqual(copy, before) {
		t.Fatal("metadata validation changed canonical content")
	}
	long := PostContent{Blocks: []Block{{Type: BlockText, Content: strings.Repeat("가", OriginMaxSourceTextChars+1)}}}
	if len(alignmentTestSave(PostContent{}, long, nil).Review.Spans) != 0 {
		t.Fatal("oversized input bypassed evidence bound")
	}
}

func TestManualPlanOriginAlignmentReordersAndEditsOneParagraphWithoutApprovalPromotion(t *testing.T) {
	before := []StorylineParagraph{{Text: "AI aroma🙂", Files: []string{"a.jpg"}}, {Text: "Owner phrase"}}
	current := PlanOriginIdentity(before)
	prior := &PlanOriginReview{Version: OriginVersion, Result: current, Sources: alignmentTestSources(), Spans: []PlanOriginSpan{{ParagraphIndex: 0, Start: 0, End: utf8.RuneCountInString(before[0].Text), Quote: before[0].Text, Category: OriginAIAdded, SourceRefs: []string{"plan"}, ReviewState: OriginConfirmed}}}
	after := []StorylineParagraph{before[1], before[0], {Text: "Explicit new paragraph"}}
	got := AlignManualPlanOrigins(before, after, prior, current, PlanOriginIdentity(after), "manual-plan", []string{"a.jpg"})
	if len(got.Spans) != 2 {
		t.Fatalf("unique plan move/new paragraph lost: %+v", got)
	}
	for _, span := range got.Spans {
		if span.Quote == "AI aroma🙂" && (span.ParagraphIndex != 1 || span.Category != OriginAIAdded || span.ReviewState != OriginConfirmed) {
			t.Fatal("plan approval/reorder promoted existing AI meaning")
		}
	}
	changed := slices.Clone(before)
	changed[0].Files = []string{"b.jpg"}
	if len(AlignManualPlanOrigins(before, changed, prior, current, PlanOriginIdentity(changed), "manual-plan", []string{"a.jpg", "b.jpg"}).Spans) != 0 {
		t.Fatal("another plan attachment inherited old proof or owner color")
	}
	prior.Result.ContentHash = "stale"
	if len(AlignManualPlanOrigins(before, before, prior, current, current, "manual-plan", []string{"a.jpg"}).Spans) != 0 {
		t.Fatal("stale old plan origins were guessed")
	}
}

func TestManualOriginAlignmentDuplicateNoopRetainsEachKnownField(t *testing.T) {
	before := PostContent{Blocks: []Block{{Type: BlockText, Content: "same"}, {Type: BlockText, Content: "same"}}}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}, "same", OriginOwnerInput, "memo"), alignmentTestCandidate(OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(1)}, "same", OriginAIAdded, "plan")})
	got := alignmentTestSave(before, before, prior, "a.jpg")
	if !reflect.DeepEqual(got.Review.Spans, prior.Spans) {
		t.Fatal("no-op chose among or discarded known duplicate field origins")
	}
}

func TestManualOriginAlignmentPreservesAllTypedReadableFieldsAndMetadataOnlyChanges(t *testing.T) {
	before := PostContent{Title: "title", Summary: "summary", Tags: []string{"tag"}, Blocks: []Block{{Type: BlockText, Content: "text"}, {Type: BlockHeading, Content: "heading", Level: 2}, {Type: BlockQuote, Content: "quote"}, {Type: BlockList, Items: []string{"item one", "item two"}}, {Type: BlockImage, File: "a.jpg", Alt: "alt", Caption: "caption"}, {Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Layout: GalleryCollage, Alt: "gallery alt", Caption: "gallery caption"}, {Type: BlockVideo, File: "v.mp4", Alt: "video alt", Caption: "video caption"}}}
	var candidates []OriginCandidate
	for _, field := range alignmentReadableFields(before) {
		text, _ := OriginFieldText(before, field)
		candidates = append(candidates, alignmentTestCandidate(field, text, OriginAIAdded, "plan"))
	}
	prior := alignmentTestReview(before, candidates)
	after := before
	after.Blocks = slices.Clone(before.Blocks)
	after.Blocks[1].Level = 3
	after.Blocks[5].Layout = GallerySlide
	got := alignmentTestSave(before, after, prior, "a.jpg", "b.jpg", "v.mp4")
	if len(got.Review.Spans) != len(candidates) {
		t.Fatalf("layout/heading formatting dropped readable-field evidence: %+v", got)
	}
	for _, span := range got.Review.Spans {
		if span.Category != OriginAIAdded || span.SourceRefs[0] != "plan" {
			t.Fatal("formatting promoted AI field to owner input")
		}
	}
}

func TestManualOriginAlignmentPureMiddleInsertDoesNotColorWholeSave(t *testing.T) {
	before := PostContent{Blocks: []Block{{Type: BlockText, Content: "AI prefix. AI suffix🙂"}}}
	first := OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentTestIndex(0)}
	prior := alignmentTestReview(before, []OriginCandidate{alignmentTestCandidate(first, "AI prefix.", OriginAIAdded, "plan"), alignmentTestCandidate(first, "AI suffix🙂", OriginAIAdded, "plan")})
	after := PostContent{Blocks: []Block{{Type: BlockText, Content: "AI prefix. Explicit new fact. AI suffix🙂"}}}
	got := alignmentTestSave(before, after, prior)
	if len(got.Review.Spans) != 3 {
		t.Fatalf("pure insert mixed origins failed: %+v", got)
	}
	owners := 0
	for _, span := range got.Review.Spans {
		if span.Category == OriginOwnerInput {
			owners++
			if !strings.Contains(span.Quote, "Explicit new fact.") || strings.Contains(span.Quote, "AI ") {
				t.Fatal("new edit source swallowed old AI meaning")
			}
		}
	}
	if owners != 1 {
		t.Fatal("pure insertion became a whole-body origin change")
	}
}
