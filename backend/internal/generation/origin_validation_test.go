package generation

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

func originInt(value int) *int { return &value }
func originSources() []post.OriginSource {
	return []post.OriginSource{
		{ID: "memo", Kind: post.OriginSourceMemo, Text: "맛있었다", Available: true},
		{ID: "answer", Kind: post.OriginSourceTemplateAnswer, Text: "explicit answer", Available: true},
		{ID: "edit", Kind: post.OriginSourceOwnerEdit, Text: "explicit new fact", Available: true},
		{ID: "memory", Kind: post.OriginSourceMemory, Text: "approved preference", Available: true},
		{ID: "photo", Kind: post.OriginSourceVisualObservation, Text: "a visible cup", AttachmentFilename: "a.jpg", Available: true},
		{ID: "video", Kind: post.OriginSourceVisualObservation, Text: "observed source-time sequence", AttachmentFilename: "a.mp4", Available: true},
		{ID: "plan", Kind: post.OriginSourceAIProposal, Text: "unconfirmed AI plan", Available: true},
	}
}
func originCandidate(field post.OriginFieldLocator, quote string, category post.OriginCategory, refs ...string) post.OriginCandidate {
	return post.OriginCandidate{Field: field, Quote: quote, Category: category, SourceRefs: refs}
}

func TestOriginSourceCategoryFenceCannotPromoteMemoMemoryOrAIPlanToVisualProof(t *testing.T) {
	content := PostContent{Title: "풍미와 장면"}
	for _, source := range originSources() {
		for _, category := range []post.OriginCategory{post.OriginOwnerInput, post.OriginPhotoInterpretation, post.OriginAIAdded} {
			t.Run(source.ID+"/"+string(category), func(t *testing.T) {
				candidate := originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "풍미", category, source.ID)
				before := originPostContent(content)
				got := ResolveWriteOriginCandidates(content, []post.OriginSource{source}, []post.OriginCandidate{candidate})
				owner := source.Kind == post.OriginSourceMemo || source.Kind == post.OriginSourceTemplateAnswer || source.Kind == post.OriginSourceOwnerEdit || source.Kind == post.OriginSourceMemory
				visual := source.Kind == post.OriginSourceVisualObservation
				allowed := category == post.OriginAIAdded || (category == post.OriginOwnerInput && owner) || (category == post.OriginPhotoInterpretation && visual)
				if (len(got.Review.Spans) == 1) != allowed {
					t.Fatalf("wrong source category fence: %+v", got)
				}
				if !reflect.DeepEqual(before, originPostContent(content)) {
					t.Fatal("origin rejection altered canonical content")
				}
			})
		}
	}
	withoutAttachment := originSources()[4]
	withoutAttachment.AttachmentFilename = ""
	got := ResolveWriteOriginCandidates(content, []post.OriginSource{withoutAttachment}, []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "풍미", post.OriginPhotoInterpretation, "photo")})
	if len(got.Review.Spans) != 0 || got.Issues[0].Code != OriginIssueSourceCategory {
		t.Fatal("unidentifiable observation became visual proof")
	}
}

func TestOriginResolutionCoversEveryReadableFieldWithKoreanEmojiAndExactOccurrences(t *testing.T) {
	content := PostContent{Title: "제목🙂", Summary: "요약", Tags: []string{"태그"}, Blocks: []Block{
		{Type: BlockText, Content: "가🙂 가🙂 맛있었다 달콤한 향"}, {Type: BlockHeading, Content: "소제목", Level: 2}, {Type: BlockQuote, Content: "인용"},
		{Type: BlockList, Items: []string{"목록 하나", "목록 둘"}}, {Type: BlockImage, File: "a.jpg", Alt: "사진 설명", Caption: "사진 캡션"},
		{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Alt: "묶음 설명", Caption: "묶음 캡션"}, {Type: BlockVideo, File: "a.mp4", Alt: "영상 설명", Caption: "영상 캡션"},
	}}
	var candidates []post.OriginCandidate
	for _, field := range readableOriginFields(content) {
		text, _ := post.OriginFieldText(originPostContent(content), field)
		if field.Kind == post.OriginFieldBlockContent && *field.BlockIndex == 0 {
			first := originCandidate(field, "가🙂", post.OriginAIAdded)
			first.Occurrence = originInt(1)
			candidates = append(candidates, first, originCandidate(field, "맛있었다", post.OriginOwnerInput, "memo"), originCandidate(field, "달콤한 향", post.OriginAIAdded, "plan"))
			continue
		}
		candidates = append(candidates, originCandidate(field, text, post.OriginAIAdded))
	}
	got := ResolveWriteOriginCandidates(content, originSources(), candidates)
	if len(got.Review.Spans) != len(candidates) || len(got.Issues) != 0 {
		t.Fatalf("readable fields lost origin spans: %+v", got)
	}
	var repeat *post.OriginSpan
	for i := range got.Review.Spans {
		if got.Review.Spans[i].Quote == "가🙂" {
			repeat = &got.Review.Spans[i]
		}
	}
	if repeat == nil || repeat.Start != 3 || repeat.End != 5 {
		t.Fatalf("emoji offsets were bytes rather than scalars: %+v", repeat)
	}
	ambiguous := originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "가🙂", post.OriginAIAdded)
	bad := append([]post.OriginCandidate{ambiguous}, originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle, BlockIndex: originInt(0)}, "제목", post.OriginAIAdded), originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTag, TagIndex: originInt(2)}, "태그", post.OriginAIAdded))
	resolved := ResolveWriteOriginCandidates(content, nil, bad)
	if len(resolved.Review.Spans) != 0 || resolved.Issues[0].Code != post.OriginIssueAmbiguous || resolved.Issues[1].Code != post.OriginIssueLocator || resolved.Issues[2].Code != post.OriginIssueLocator {
		t.Fatalf("invalid locators/repeats silently selected text: %+v", resolved)
	}
}

func TestOriginResolutionKeepsCanonicalContentForMissingMalformedSourcesAndConflicts(t *testing.T) {
	content := PostContent{Title: "abcdef", Summary: "safe"}
	copy := originPostContent(content)
	got := ResolveWriteOriginCandidates(content, nil, nil)
	if len(got.Review.Spans) != 0 || !reflect.DeepEqual(copy, originPostContent(content)) {
		t.Fatal("missing origins discarded content")
	}
	candidates := []post.OriginCandidate{
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "abcde", post.OriginAIAdded),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "cd", post.OriginAIAdded),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldSummary}, "safe", post.OriginAIAdded),
	}
	got = ResolveWriteOriginCandidates(content, nil, candidates)
	if len(got.Review.Spans) != 1 || got.Review.Spans[0].Field.Kind != post.OriginFieldSummary {
		t.Fatal("conflicting spans were repaired/guessed")
	}
	badCatalog := append(originSources(), originSources()[0])
	got = ResolveWriteOriginCandidates(content, badCatalog, candidates)
	if len(got.Review.Spans) != 0 || got.Issues[0].Code != post.OriginIssueSourceCatalog || !reflect.DeepEqual(copy, originPostContent(content)) {
		t.Fatal("catalog validation changed canonical output")
	}
	unavailable := originSources()[0]
	unavailable.Available = false
	got = ResolveWriteOriginCandidates(content, []post.OriginSource{unavailable}, []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "abc", post.OriginOwnerInput, "memo")})
	if len(got.Review.Spans) != 0 || got.Issues[0].Code != post.OriginIssueUnavailable {
		t.Fatal("withdrawn evidence remained proof")
	}
}

func TestOriginFinalizationExplicitlyMapsDroppedBlocksAndGalleryCaptionCopies(t *testing.T) {
	content := PostContent{Title: "title", Tags: []string{"tag"}, Blocks: []Block{
		{Type: BlockText, Content: "same", File: "invalid-field"},
		{Type: BlockText, Content: "same"},
		{Type: BlockImage, File: "withdrawn.jpg", Alt: "same", Caption: "deleted"},
		{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg", "unknown.jpg", "a.jpg"}, Caption: "  첫 캡션🙂  ", Alt: "대체 문구"},
		{Type: BlockList, Items: []string{"first", "second"}},
		{Type: BlockHeading, Content: "heading", Level: 9},
	}}
	candidates := []post.OriginCandidate{
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "same", post.OriginOwnerInput, "memo"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(1)}, "same", post.OriginAIAdded),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockAlt, BlockIndex: originInt(2)}, "same", post.OriginOwnerInput, "memo"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockCaption, BlockIndex: originInt(3)}, "첫 캡션🙂", post.OriginOwnerInput, "memo"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockAlt, BlockIndex: originInt(3)}, "대체 문구", post.OriginPhotoInterpretation, "photo"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockItem, BlockIndex: originInt(4), ItemIndex: originInt(1)}, "second", post.OriginAIAdded),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(5)}, "heading", post.OriginAIAdded),
	}
	photos := []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"}
	portraits := map[string]bool{"c.jpg": true, "d.jpg": true}
	want := content
	want.Blocks = ValidateBlocks(want.Blocks)
	want = FilterAttachments(want, photos, nil, portraits)
	got := FinalizeWriteOrigins(WriteAnswer{Content: content, OriginCandidates: candidates}, photos, nil, portraits, originSources())
	if !reflect.DeepEqual(got.Content, want) {
		t.Fatalf("metadata changed canonical normalization: got=%+v want=%+v", got.Content, want)
	}
	for _, span := range got.Origins.Spans {
		if span.Quote == "same" && (span.Category != post.OriginAIAdded || *span.Field.BlockIndex != 0) {
			t.Fatal("dropped evidence attached to same quote in surviving block")
		}
		if span.Quote == "첫 캡션🙂" && (*span.Field.BlockIndex != 1 || span.Start != 0) {
			t.Fatal("caption trimming/split mapped to wrong field")
		}
		if span.Quote == "대체 문구" && span.Field.Kind == post.OriginFieldBlockCaption && *span.Field.BlockIndex == 1 {
			t.Fatal("alt proof recolored first written caption")
		}
	}
	if got.Content.Blocks[len(got.Content.Blocks)-1].Level != 2 {
		t.Fatal("heading normalization changed")
	}
	if got.Origins.Result.ContentHash != OriginContentIdentity(got.Content).ContentHash {
		t.Fatal("origin review attached to pre-normalization result")
	}
}

func TestPlanOriginMapDoesNotPromoteApprovalOrSelectAmbiguousTruncatedRepeat(t *testing.T) {
	if RemapPlanOriginCandidates(nil, nil, nil, nil) != nil || RemapPlanOriginCandidates(nil, nil, nil, []PlanOriginCandidate{}) != nil {
		t.Fatal("legacy empty origin candidates changed canonical annotation shape")
	}
	raw := []StorylineParagraph{{Text: ""}, {Text: "  가🙂 가🙂  "}, {Text: ""}, {Text: "앞" + strings.Repeat("나", StorylineTextMaxChars) + "앞"}}
	normalized := []StorylineParagraph{{Text: "가🙂 가🙂"}, {Text: "앞" + strings.Repeat("나", StorylineTextMaxChars-1)}}
	mapping := []int{-1, 0, -1, 1}
	candidates := []PlanOriginCandidate{
		{ParagraphIndex: 1, Quote: "가🙂", Occurrence: originInt(1), Category: post.OriginPhotoInterpretation, SourceRefs: []string{"photo"}},
		{ParagraphIndex: 3, Quote: "앞", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}},
		{ParagraphIndex: 0, Quote: "가🙂", Category: post.OriginAIAdded},
		{ParagraphIndex: 1, Quote: "가🙂", Occurrence: originInt(0), Category: post.OriginOwnerInput, SourceRefs: []string{"plan"}},
	}
	mapped := RemapPlanOriginCandidates(raw, normalized, mapping, candidates)
	if mapped[0].ParagraphIndex != 0 || *mapped[0].Occurrence != 1 || mapped[1].ParagraphIndex != -1 || mapped[2].ParagraphIndex != -1 {
		t.Fatal("plan normalization guessed a raw occurrence or paragraph")
	}
	got := ResolvePlanOriginCandidates(normalized, originSources(), mapped)
	if len(got.Review.Spans) != 1 || got.Review.Spans[0].Start != 3 || got.Review.Spans[0].Category != post.OriginPhotoInterpretation || got.Review.Result.ContentHash == "" {
		t.Fatalf("plan provenance or scalar coordinates lost: %+v", got)
	}
	changed := slices.Clone(normalized)
	changed[0].Text = "owner edited paragraph"
	if ValidatePlanOrigins(changed, originSources(), mapped).Result == got.Review.Result {
		t.Fatal("edited plan reused previous result identity")
	}
}

func TestRevisionOriginsRetainExactOriginalMeaningAndRejectUntouchedRecoloring(t *testing.T) {
	current := PostContent{Title: "AI 제목", Summary: "old", Blocks: []Block{{Type: BlockText, Content: "달콤한 향"}, {Type: BlockImage, File: "a.jpg", Alt: "cup", Caption: "caption"}}}
	priorCandidates := []post.OriginCandidate{
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "AI 제목", post.OriginAIAdded, "plan"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "달콤한 향", post.OriginAIAdded, "plan"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockAlt, BlockIndex: originInt(1)}, "cup", post.OriginPhotoInterpretation, "photo"),
	}
	prior := ResolveWriteOriginCandidates(current, originSources(), priorCandidates).Review
	after := current
	after.Summary = "new fact"
	after.Blocks = slices.Clone(current.Blocks)
	after.Blocks[1].File = "b.jpg"
	newCandidates := []post.OriginCandidate{
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "AI 제목", post.OriginOwnerInput, "memo"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "달콤한 향", post.OriginOwnerInput, "memo"),
		originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldSummary}, "new fact", post.OriginOwnerInput, "edit"),
	}
	got := PreserveRevisionOrigins(current, &prior, after, originSources(), newCandidates)
	if len(got.Spans) != 3 {
		t.Fatalf("untouched provenance or explicit new input lost: %+v", got)
	}
	for _, span := range got.Spans {
		if span.Field.Kind != post.OriginFieldSummary && span.Category != post.OriginAIAdded {
			t.Fatal("unchanged AI meaning promoted to owner input")
		}
	}
	withoutPrior := PreserveRevisionOrigins(current, nil, after, originSources(), newCandidates)
	if len(withoutPrior.Spans) != 1 {
		t.Fatal("missing original origins were invented from current memo")
	}
	prior.Result.ContentRevision = 9
	stale := PreserveRevisionOrigins(current, &prior, after, originSources(), newCandidates)
	if len(stale.Spans) != 1 {
		t.Fatal("unknown durable prior result was guessed")
	}
	identity := prior.Result
	known := PreserveRevisionOrigins(current, &prior, after, originSources(), newCandidates, identity)
	if len(known.Spans) != 3 {
		t.Fatal("authenticated exact durable prior result was not retained")
	}
	identity.ContentRevision++
	if len(PreserveRevisionOrigins(current, &prior, after, originSources(), newCandidates, identity).Spans) != 1 {
		t.Fatal("stale durable identity survived")
	}
}

func TestRevisionOriginsKeepFrozenEvidenceWhenMemoIDChangesAndFenceWithdrawnMedia(t *testing.T) {
	current := PostContent{Title: "supplied meaning", Blocks: []Block{{Type: BlockText, Content: "visible cup"}}, Summary: "old"}
	sources := originSources()
	prior := ResolveWriteOriginCandidates(current, sources, []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "supplied meaning", post.OriginOwnerInput, "memo"), originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "visible cup", post.OriginPhotoInterpretation, "photo")}).Review
	after := current
	after.Summary = "new fact"
	newSources := slices.Clone(sources)
	newSources[0].Text = "a newer unrelated memo"
	newSources[4].Available = false
	got := PreserveRevisionOrigins(current, &prior, after, newSources, []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldSummary}, "new fact", post.OriginOwnerInput, "memo")})
	if len(got.Spans) != 2 {
		t.Fatalf("retained old evidence or withdrawal fence lost: %+v", got)
	}
	var oldRef string
	for _, span := range got.Spans {
		if span.Field.Kind == post.OriginFieldTitle {
			oldRef = span.SourceRefs[0]
		}
		if span.Quote == "visible cup" {
			t.Fatal("known withdrawn visual source was restored")
		}
	}
	if oldRef == "" || oldRef == "memo" {
		t.Fatal("old meaning silently retargeted to newer memo")
	}
	var retainedText string
	for _, source := range got.Sources {
		if source.ID == oldRef {
			retainedText = source.Text
		}
	}
	if retainedText != "맛있었다" {
		t.Fatal("retained source was reconstructed from new settings")
	}
	fenced := OriginSourcesWithAttachments(sources, nil, []string{"a.mp4"})
	if fenced[4].Available || !fenced[5].Available || !sources[4].Available {
		t.Fatal("fresh attachment fence guessed or mutated frozen source")
	}
}

func TestNormalizationMapDropsOnlyUnprovedAnnotationsAndNeverUsesSiblingQuote(t *testing.T) {
	before := PostContent{Title: "title", Blocks: []Block{{Type: BlockText, Content: "same"}, {Type: BlockText, Content: "same"}}}
	after := PostContent{Title: "title", Blocks: []Block{{Type: BlockText, Content: "same"}}}
	candidate := originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, "same", post.OriginOwnerInput, "memo")
	mapping := OriginFieldMap{{From: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(1)}, To: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}, SourceStart: 0, SourceEnd: 4}}
	got, issues := RemapOriginCandidates(before, after, mapping, []post.OriginCandidate{candidate, originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "title", post.OriginAIAdded)})
	if len(got) != 0 || len(issues) != 2 || issues[0].Code != OriginIssueNormalization {
		t.Fatal("same quote in sibling attracted original evidence")
	}
	mapping = append(mapping, OriginFieldCorrespondence{From: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, To: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, SourceStart: 0, SourceEnd: 5})
	got, issues = RemapOriginCandidates(before, after, mapping, []post.OriginCandidate{candidate, originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldTitle}, "title", post.OriginAIAdded)})
	if len(got) != 1 || got[0].Field.Kind != post.OriginFieldTitle || len(issues) != 1 {
		t.Fatal("unproved normalization degraded unaffected annotation")
	}
}

func TestGalleryOriginMapFollowsActualFallbackTextInsteadOfEqualOtherField(t *testing.T) {
	for _, tc := range []struct {
		caption, alt, quote string
		from                post.OriginFieldKind
		wantCaptionParts    int
	}{{" caption ", "alt", "caption", post.OriginFieldBlockCaption, 1}, {"", "alt", "alt", post.OriginFieldBlockAlt, 2}, {" caption ", "  ", "caption", post.OriginFieldBlockCaption, 2}} {
		before := PostContent{Blocks: []Block{{Type: BlockGallery, Files: []string{"a", "b", "c", "d"}, Caption: tc.caption, Alt: tc.alt}}}
		after, mapping := NormalizeContentWithOriginMap(before, []string{"a", "b", "c", "d"}, nil, nil)
		candidates, issues := RemapOriginCandidates(before, after, mapping, []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: tc.from, BlockIndex: originInt(0)}, tc.quote, post.OriginAIAdded)})
		if len(issues) != 0 {
			t.Fatalf("proven caption copy lost: %+v", issues)
		}
		count := 0
		for _, candidate := range candidates {
			if candidate.Field.Kind == post.OriginFieldBlockCaption {
				count++
			}
		}
		if count != tc.wantCaptionParts {
			t.Fatalf("caption fallback inherited wrong field: caption=%q alt=%q candidates=%+v", tc.caption, tc.alt, candidates)
		}
	}
}

func TestRevisionOriginRangesRetainUntouchedAITailWithShiftedScalarsAndConfirmation(t *testing.T) {
	current := PostContent{Blocks: []Block{{Type: BlockText, Content: "처음 문장은 오래된 내용입니다. 달콤한 향🙂은 AI 제안입니다."}}}
	field := post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}
	prior := ResolveWriteOriginCandidates(current, originSources(), []post.OriginCandidate{originCandidate(field, "달콤한 향🙂", post.OriginAIAdded, "plan")}).Review
	prior.Spans[0].ReviewState = post.OriginConfirmed
	after := PostContent{Blocks: []Block{{Type: BlockText, Content: "새 사실입니다. 달콤한 향🙂은 AI 제안입니다."}}}
	newCandidates := []post.OriginCandidate{originCandidate(field, "새 사실입니다.", post.OriginOwnerInput, "edit"), originCandidate(field, "달콤한 향🙂", post.OriginOwnerInput, "memo")}
	got := PreserveRevisionOrigins(current, &prior, after, originSources(), newCandidates)
	if len(got.Spans) != 2 {
		t.Fatalf("untouched tail lost to recoloring: %+v", got)
	}
	var retained *post.OriginSpan
	for i := range got.Spans {
		if got.Spans[i].Quote == "달콤한 향🙂" {
			retained = &got.Spans[i]
		}
	}
	if retained == nil || retained.Category != post.OriginAIAdded || retained.ReviewState != post.OriginConfirmed || !slices.Equal(retained.SourceRefs, []string{"plan"}) || retained.Start != 9 || retained.End != 15 {
		t.Fatalf("suffix text/source/state/Unicode shift lost: %+v", retained)
	}
	if len(PreserveRevisionOrigins(current, nil, after, originSources(), newCandidates).Spans) != 1 {
		t.Fatal("missing original tail was relabeled from newer memo")
	}
	whole := originCandidate(field, after.Blocks[0].Content, post.OriginOwnerInput, "memo")
	collision := PreserveRevisionOrigins(current, &prior, after, originSources(), append(newCandidates, whole))
	if len(collision.Spans) != 2 {
		t.Fatal("new whole-field color poisoned known unchanged tail or new fact")
	}
}

func TestRevisionOriginRangesKeepPrefixAndLeaveInteriorRepeatUnproved(t *testing.T) {
	field := post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: originInt(0)}
	current := PostContent{Blocks: []Block{{Type: BlockText, Content: "확실한 시작🙂. old one. 중간 문구. old end."}}}
	prior := ResolveWriteOriginCandidates(current, originSources(), []post.OriginCandidate{originCandidate(field, "확실한 시작🙂", post.OriginPhotoInterpretation, "photo"), originCandidate(field, "중간 문구", post.OriginAIAdded, "plan")}).Review
	after := PostContent{Blocks: []Block{{Type: BlockText, Content: "확실한 시작🙂. new one. 중간 문구. new ending."}}}
	got := PreserveRevisionOrigins(current, &prior, after, originSources(), nil)
	if len(got.Spans) != 1 || got.Spans[0].Quote != "확실한 시작🙂" || got.Spans[0].Start != 0 {
		t.Fatal("prefix was lost or interior similarity guessed")
	}
	after.Blocks[0].Type = BlockQuote
	if len(PreserveRevisionOrigins(current, &prior, after, originSources(), nil).Spans) != 0 {
		t.Fatal("typed field identity changed but old proof survived")
	}
}

func TestPlanOriginsPreserveUniqueReorderedParagraphsWithoutRecoloringOrApprovalPromotion(t *testing.T) {
	before := []StorylineParagraph{{Text: "AI aroma🙂", Files: []string{"a.jpg"}}, {Text: "supplied meaning", Files: []string{"b.jpg"}}}
	prior := ValidatePlanOrigins(before, originSources(), []PlanOriginCandidate{{ParagraphIndex: 0, Quote: "AI aroma🙂", Category: post.OriginAIAdded, SourceRefs: []string{"plan"}}, {ParagraphIndex: 1, Quote: "supplied meaning", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}}})
	prior.Spans[0].ReviewState = post.OriginConfirmed
	after := []StorylineParagraph{before[1], before[0], {Text: "new explicit fact"}}
	newCandidates := []PlanOriginCandidate{{ParagraphIndex: 1, Quote: "AI aroma🙂", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}}, {ParagraphIndex: 2, Quote: "new explicit fact", Category: post.OriginOwnerInput, SourceRefs: []string{"edit"}}}
	got := PreservePlanOrigins(before, &prior, after, originSources(), newCandidates)
	if len(got.Spans) != 3 {
		t.Fatalf("known reordered plan spans lost: %+v", got)
	}
	var ai *PlanOriginSpan
	for i := range got.Spans {
		if got.Spans[i].Quote == "AI aroma🙂" {
			ai = &got.Spans[i]
		}
	}
	if ai == nil || ai.ParagraphIndex != 1 || ai.Category != post.OriginAIAdded || ai.ReviewState != post.OriginConfirmed || !slices.Equal(ai.SourceRefs, []string{"plan"}) {
		t.Fatal("plan arrangement or candidate promoted old AI meaning")
	}
	if len(PreservePlanOrigins(before, nil, after, originSources(), newCandidates).Spans) != 1 {
		t.Fatal("missing old plan provenance inferred from new memo")
	}
	prior.Result.ContentHash = "stale"
	if len(PreservePlanOrigins(before, &prior, after, originSources(), newCandidates).Spans) != 1 {
		t.Fatal("stale old plan evidence restored")
	}
}

func TestPlanOriginsDoNotChooseDuplicateParagraphOrDifferentFiles(t *testing.T) {
	before := []StorylineParagraph{{Text: "same", Files: []string{"a.jpg"}}, {Text: "same", Files: []string{"a.jpg"}}, {Text: "unique", Files: []string{"a.jpg"}}}
	prior := ValidatePlanOrigins(before, originSources(), []PlanOriginCandidate{{ParagraphIndex: 0, Quote: "same", Category: post.OriginAIAdded}, {ParagraphIndex: 2, Quote: "unique", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"photo"}}})
	after := []StorylineParagraph{{Text: "same", Files: []string{"a.jpg"}}, {Text: "unique", Files: []string{"b.jpg"}}}
	candidates := []PlanOriginCandidate{{ParagraphIndex: 0, Quote: "same", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}}, {ParagraphIndex: 1, Quote: "unique", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}}}
	if len(PreservePlanOrigins(before, &prior, after, originSources(), candidates).Spans) != 0 {
		t.Fatal("ambiguous paragraph/dependent files were guessed or recolored")
	}
}
