package generation

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/post"
)

// OriginFieldCorrespondence records a proven copy/subrange made by canonical
// normalization. SourceStart/End are scalar positions in the original field;
// that exact range is the whole target field. No quote search chooses a target.
type OriginFieldCorrespondence struct {
	From, To               post.OriginFieldLocator
	SourceStart, SourceEnd int
}

type OriginFieldMap []OriginFieldCorrespondence

type originFieldKey struct {
	kind             post.OriginFieldKind
	tag, block, item int
}

func originKey(field post.OriginFieldLocator) originFieldKey {
	value := func(index *int) int {
		if index == nil {
			return -1
		}
		return *index
	}
	return originFieldKey{field.Kind, value(field.TagIndex), value(field.BlockIndex), value(field.ItemIndex)}
}
func originLocatorEqual(left, right post.OriginFieldLocator) bool {
	return originKey(left) == originKey(right)
}
func cloneOriginIndex(index *int) *int {
	if index == nil {
		return nil
	}
	value := *index
	return &value
}
func cloneOriginLocator(field post.OriginFieldLocator) post.OriginFieldLocator {
	field.TagIndex, field.BlockIndex, field.ItemIndex = cloneOriginIndex(field.TagIndex), cloneOriginIndex(field.BlockIndex), cloneOriginIndex(field.ItemIndex)
	return field
}

func readableOriginFields(content PostContent) []post.OriginFieldLocator {
	fields := []post.OriginFieldLocator{{Kind: post.OriginFieldTitle}, {Kind: post.OriginFieldSummary}}
	for index := range content.Tags {
		i := index
		fields = append(fields, post.OriginFieldLocator{Kind: post.OriginFieldTag, TagIndex: &i})
	}
	for index, block := range content.Blocks {
		i := index
		switch block.Type {
		case BlockText, BlockHeading, BlockQuote:
			fields = append(fields, post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &i})
		case BlockList:
			for item := range block.Items {
				j := item
				fields = append(fields, post.OriginFieldLocator{Kind: post.OriginFieldBlockItem, BlockIndex: &i, ItemIndex: &j})
			}
		case BlockImage, BlockGallery, BlockVideo:
			fields = append(fields, post.OriginFieldLocator{Kind: post.OriginFieldBlockAlt, BlockIndex: &i}, post.OriginFieldLocator{Kind: post.OriginFieldBlockCaption, BlockIndex: &i})
		}
	}
	return fields
}

// NormalizeContentWithOriginMap uses the existing validators at the exact block
// seam. Invalid blocks/attachments disappear explicitly, and gallery splits keep
// their originating block and the actual caption/alt copy decision.
func NormalizeContentWithOriginMap(content PostContent, photos, videos []string, portrait map[string]bool) (PostContent, OriginFieldMap) {
	before := originPostContent(content)
	after := PostContent{Title: content.Title, Summary: content.Summary, Tags: slices.Clone(content.Tags)}
	var mapping OriginFieldMap
	add := func(from, to post.OriginFieldLocator, start, end int) {
		mapping = append(mapping, OriginFieldCorrespondence{From: cloneOriginLocator(from), To: cloneOriginLocator(to), SourceStart: start, SourceEnd: end})
	}
	for _, field := range readableOriginFields(PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags}) {
		text, _ := post.OriginFieldText(before, field)
		add(field, field, 0, utf8.RuneCountInString(text))
	}
	attachedPhotos, attachedVideos := nameSet(photos), nameSet(videos)
	for oldIndex, block := range content.Blocks {
		validated := ValidateBlocks([]Block{block})
		if len(validated) == 0 {
			continue
		}
		canonical := validated[0]
		var parts []Block
		switch canonical.Type {
		case BlockGallery:
			parts = filterGallery(canonical, attachedPhotos, portrait)
		case BlockImage:
			if _, exists := attachedPhotos[canonical.File]; exists {
				parts = []Block{canonical}
			}
		case BlockVideo:
			if _, exists := attachedVideos[canonical.File]; exists {
				parts = []Block{canonical}
			}
		default:
			parts = []Block{canonical}
		}
		for partIndex, part := range parts {
			newIndex := len(after.Blocks)
			after.Blocks = append(after.Blocks, part)
			oldLocator := func(kind post.OriginFieldKind) post.OriginFieldLocator {
				i := oldIndex
				return post.OriginFieldLocator{Kind: kind, BlockIndex: &i}
			}
			newLocator := func(kind post.OriginFieldKind) post.OriginFieldLocator {
				i := newIndex
				return post.OriginFieldLocator{Kind: kind, BlockIndex: &i}
			}
			if canonical.Type == BlockGallery {
				add(oldLocator(post.OriginFieldBlockAlt), newLocator(post.OriginFieldBlockAlt), 0, utf8.RuneCountInString(block.Alt))
				kind := post.OriginFieldBlockCaption
				start, end := trimmedOriginRange(block.Caption)
				if (partIndex == 0 && strings.TrimSpace(block.Caption) == "") || (partIndex > 0 && strings.TrimSpace(block.Alt) != "") {
					kind, start, end = post.OriginFieldBlockAlt, 0, utf8.RuneCountInString(block.Alt)
				}
				// The later-part fallback follows the first caption, which itself
				// may have been copied from an all-whitespace alt.
				if partIndex > 0 && strings.TrimSpace(block.Alt) == "" && strings.TrimSpace(block.Caption) == "" {
					kind, start, end = post.OriginFieldBlockAlt, 0, utf8.RuneCountInString(block.Alt)
				}
				add(oldLocator(kind), newLocator(post.OriginFieldBlockCaption), start, end)
				continue
			}
			for _, field := range readableOriginFields(PostContent{Blocks: []Block{canonical}}) {
				if field.BlockIndex == nil {
					continue
				}
				from, to := cloneOriginLocator(field), cloneOriginLocator(field)
				from.BlockIndex, to.BlockIndex = cloneOriginIndex(&oldIndex), cloneOriginIndex(&newIndex)
				text, ok := post.OriginFieldText(before, from)
				if ok {
					add(from, to, 0, utf8.RuneCountInString(text))
				}
			}
		}
	}
	return after, mapping
}

func trimmedOriginRange(text string) (int, int) {
	start := utf8.RuneCountInString(text) - utf8.RuneCountInString(strings.TrimLeftFunc(text, unicode.IsSpace))
	trimmed := strings.TrimSpace(text)
	return start, start + utf8.RuneCountInString(trimmed)
}

// RemapOriginCandidates first resolves a candidate in its original field. The
// explicit normalization map must prove the target bytes and same scalar range;
// identical quotes in unrelated surviving blocks can never attract a lost span.
func RemapOriginCandidates(before, after PostContent, mapping OriginFieldMap, candidates []post.OriginCandidate) ([]post.OriginCandidate, []post.OriginIssue) {
	var result []post.OriginCandidate
	var issues []post.OriginIssue
	beforeContent, afterContent := originPostContent(before), originPostContent(after)
	for index, candidate := range candidates {
		oldText, fieldValid := post.OriginFieldText(beforeContent, candidate.Field)
		if !fieldValid {
			issues = append(issues, post.OriginIssue{Index: index, Code: post.OriginIssueLocator})
			continue
		}
		probe := candidate
		probe.Field = post.OriginFieldLocator{Kind: post.OriginFieldTitle}
		probe.Category, probe.SourceRefs = post.OriginAIAdded, nil
		resolved := post.ResolveOriginCandidates(post.PostContent{Title: oldText}, post.OriginResultIdentity{ContentHash: "original-field"}, nil, []post.OriginCandidate{probe})
		if len(resolved.Review.Spans) != 1 {
			for _, issue := range resolved.Issues {
				issues = append(issues, post.OriginIssue{Index: index, Code: issue.Code})
			}
			continue
		}
		span := resolved.Review.Spans[0]
		matched := false
		for _, copy := range mapping {
			if !originLocatorEqual(copy.From, candidate.Field) || span.Start < copy.SourceStart || span.End > copy.SourceEnd {
				continue
			}
			newText, valid := post.OriginFieldText(afterContent, copy.To)
			runes := []rune(oldText)
			if !valid || copy.SourceStart < 0 || copy.SourceEnd < copy.SourceStart || copy.SourceEnd > len(runes) || string(runes[copy.SourceStart:copy.SourceEnd]) != newText {
				continue
			}
			occurrence, valid := exactOriginOccurrence(newText, candidate.Quote, span.Start-copy.SourceStart)
			if !valid {
				continue
			}
			next := candidate
			next.Field, next.Occurrence, next.SourceRefs = cloneOriginLocator(copy.To), &occurrence, slices.Clone(candidate.SourceRefs)
			result = append(result, next)
			matched = true
		}
		if !matched {
			issues = append(issues, post.OriginIssue{Index: index, Code: OriginIssueNormalization})
		}
	}
	return result, issues
}

func exactOriginOccurrence(text, quote string, scalarStart int) (int, bool) {
	if quote == "" || !utf8.ValidString(text) || !utf8.ValidString(quote) || scalarStart < 0 {
		return 0, false
	}
	position, occurrence := 0, 0
	for position < len(text) {
		relative := strings.Index(text[position:], quote)
		if relative < 0 {
			break
		}
		match := position + relative
		if utf8.RuneCountInString(text[:match]) == scalarStart {
			return occurrence, true
		}
		position, occurrence = match+len(quote), occurrence+1
	}
	return 0, false
}

// RemapPlanOriginCandidates retains the raw paragraph's exact occurrence through
// the parser's explicit old-to-new indices and its existing trim/prefix cap.
// A rejected candidate keeps an invalid locator, so later validation cannot turn
// an originally ambiguous repeated quote into a unique surviving match.
func RemapPlanOriginCandidates(raw, normalized []StorylineParagraph, oldToNew []int, candidates []PlanOriginCandidate) []PlanOriginCandidate {
	if len(candidates) == 0 {
		return nil
	}
	result := make([]PlanOriginCandidate, len(candidates))
	for index, candidate := range candidates {
		next := candidate
		next.SourceRefs, next.Occurrence = slices.Clone(candidate.SourceRefs), cloneOriginIndex(candidate.Occurrence)
		next.ParagraphIndex = -1
		old := candidate.ParagraphIndex
		if old < 0 || old >= len(raw) || old >= len(oldToNew) {
			result[index] = next
			continue
		}
		mapped := oldToNew[old]
		if mapped < 0 || mapped >= len(normalized) {
			result[index] = next
			continue
		}
		probe := post.OriginCandidate{Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Quote: candidate.Quote, Occurrence: candidate.Occurrence, Category: post.OriginAIAdded}
		resolved := post.ResolveOriginCandidates(post.PostContent{Title: raw[old].Text}, post.OriginResultIdentity{ContentHash: "original-plan-paragraph"}, nil, []post.OriginCandidate{probe})
		if len(resolved.Review.Spans) != 1 {
			result[index] = next
			continue
		}
		span := resolved.Review.Spans[0]
		start, end := trimmedOriginRange(raw[old].Text)
		end = min(end, start+StorylineTextMaxChars)
		runes := []rune(raw[old].Text)
		if span.Start < start || span.End > end || string(runes[start:end]) != normalized[mapped].Text {
			result[index] = next
			continue
		}
		occurrence, ok := exactOriginOccurrence(normalized[mapped].Text, candidate.Quote, span.Start-start)
		if ok {
			next.ParagraphIndex, next.Occurrence = mapped, &occurrence
		}
		result[index] = next
	}
	return result
}
