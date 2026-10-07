package post

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"unicode"
	"unicode/utf8"
)

// AlignManualOriginReview uses only the two canonical results and the explicitly
// supplied current identity. It never infers a missing old origin from new text.
// The owner-input ID and attachment list are server-owned facts supplied by the
// authenticated save transaction, rather than trusted annotation labels.
func AlignManualOriginReview(before, after PostContent, prior *OriginReview, current, next OriginResultIdentity, ownerInputID string, attachedFilenames []string) OriginResolution {
	frozen := alignmentAvailableSources(prior, attachedFilenames)
	validated := ValidateOriginReview(before, current, frozen)
	result := OriginResolution{Review: OriginReview{Version: OriginVersion, Result: next, Sources: slices.Clone(validated.Review.Sources)}, Issues: slices.Clone(validated.Issues)}
	pairs, ambiguous := alignmentFieldPairs(before, after)
	catalog := map[string]OriginSource{}
	for _, source := range result.Review.Sources {
		catalog[source.ID] = source
	}
	for index, span := range validated.Review.Spans {
		if !alignmentSourceCompatible(before, span, catalog) {
			result.Issues = append(result.Issues, OriginIssue{Index: index, Code: OriginIssueCategory})
			continue
		}
		pair, exists := pairs[originKey(span.Field)]
		if !exists {
			continue
		}
		for _, region := range pair.regions {
			if span.Start >= region.oldStart && span.End <= region.oldEnd {
				kept := cloneOriginSpan(span)
				kept.Field = alignmentCloneField(pair.to)
				kept.Start = region.newStart + span.Start - region.oldStart
				kept.End = kept.Start + span.End - span.Start
				result.Review.Spans = append(result.Review.Spans, kept)
				break
			}
		}
	}
	mapped := map[originFieldKey]alignmentFieldPair{}
	for _, pair := range pairs {
		mapped[originKey(pair.to)] = pair
	}
	for _, field := range alignmentReadableFields(after) {
		if ambiguous[originKey(field)] {
			continue
		}
		text, valid := OriginFieldText(after, field)
		if !valid || !utf8.ValidString(text) {
			continue
		}
		insertions := [][2]int{{0, utf8.RuneCountInString(text)}}
		if pair, exists := mapped[originKey(field)]; exists {
			old, ok := OriginFieldText(before, pair.from)
			if !ok {
				continue
			}
			oldCount := utf8.RuneCountInString(old)
			retained := 0
			for _, region := range pair.regions {
				retained += region.oldEnd - region.oldStart
			}
			// Only a pure insertion leaves every original scalar intact. Replacement
			// of old meaning does not acquire owner provenance merely on Save.
			if retained != oldCount {
				continue
			}
			insertions = nil
			cursor := 0
			for _, region := range pair.regions {
				if region.newStart > cursor {
					insertions = append(insertions, [2]int{cursor, region.newStart})
				}
				cursor = max(cursor, region.newEnd)
			}
			if cursor < utf8.RuneCountInString(text) {
				insertions = append(insertions, [2]int{cursor, utf8.RuneCountInString(text)})
			}
		}
		for _, insertion := range insertions {
			start, end := insertion[0], insertion[1]
			if start < 0 || end <= start {
				continue
			}
			runes := []rune(text)
			quote := string(runes[start:end])
			if !alignmentMeaningfulInput(quote) || ownerInputID == "" {
				continue
			}
			if len(result.Review.Sources) >= OriginMaxSources || utf8.RuneCountInString(quote) > OriginMaxSourceTextChars {
				result.Issues = append(result.Issues, OriginIssue{Index: -1, Code: OriginIssueMetadataLimit})
				continue
			}
			id := fmt.Sprintf("%s.%d", ownerInputID, len(result.Review.Sources))
			if !utf8.ValidString(id) || utf8.RuneCountInString(id) > OriginMaxSourceIDChars {
				result.Issues = append(result.Issues, OriginIssue{Index: -1, Code: OriginIssueMetadataLimit})
				continue
			}
			if _, duplicate := catalog[id]; duplicate {
				result.Issues = append(result.Issues, OriginIssue{Index: -1, Code: OriginIssueSourceCatalog})
				continue
			}
			source := OriginSource{ID: id, Kind: OriginSourceOwnerEdit, Text: quote, Available: true}
			result.Review.Sources = append(result.Review.Sources, source)
			catalog[id] = source
			result.Review.Spans = append(result.Review.Spans, OriginSpan{Field: alignmentCloneField(field), Start: start, End: end, Quote: quote, Category: OriginOwnerInput, SourceRefs: []string{id}, ReviewState: OriginUnreviewed})
		}
	}

	final := ValidateOriginReview(after, next, &result.Review)
	final.Issues = append(result.Issues, final.Issues...)
	return final
}

func alignmentAvailableSources(prior *OriginReview, attached []string) *OriginReview {
	if prior == nil {
		return nil
	}
	copy := *prior
	copy.Sources = slices.Clone(prior.Sources)
	for i := range copy.Sources {
		if copy.Sources[i].Kind == OriginSourceVisualObservation && !slices.Contains(attached, copy.Sources[i].AttachmentFilename) {
			copy.Sources[i].Available = false
		}
	}
	return &copy
}

func alignmentSourceCompatible(content PostContent, span OriginSpan, catalog map[string]OriginSource) bool {
	for _, ref := range span.SourceRefs {
		source, exists := catalog[ref]
		if !exists || !source.Available {
			return false
		}
		switch span.Category {
		case OriginOwnerInput:
			switch source.Kind {
			case OriginSourceMemo, OriginSourceTemplateAnswer, OriginSourceOwnerEdit, OriginSourceMemory, OriginSourceLiteralText:
			default:
				return false
			}
		case OriginPhotoInterpretation:
			if source.Kind != OriginSourceVisualObservation || source.AttachmentFilename == "" {
				return false
			}
			if span.Field.Kind == OriginFieldBlockAlt || span.Field.Kind == OriginFieldBlockCaption {
				if span.Field.BlockIndex == nil || *span.Field.BlockIndex < 0 || *span.Field.BlockIndex >= len(content.Blocks) {
					return false
				}
				block := content.Blocks[*span.Field.BlockIndex]
				files := []string{block.File}
				if block.Type == BlockGallery {
					files = block.Files
				}
				if !slices.Contains(files, source.AttachmentFilename) {
					return false
				}
			}
		case OriginAIAdded:
		default:
			return false
		}
	}
	return span.Category == OriginAIAdded || len(span.SourceRefs) > 0
}

func alignmentMeaningfulInput(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

func alignmentCloneField(field OriginFieldLocator) OriginFieldLocator {
	clone := func(index *int) *int {
		if index == nil {
			return nil
		}
		value := *index
		return &value
	}
	field.TagIndex, field.BlockIndex, field.ItemIndex = clone(field.TagIndex), clone(field.BlockIndex), clone(field.ItemIndex)
	return field
}

type alignmentRegion struct{ oldStart, oldEnd, newStart, newEnd int }
type alignmentFieldPair struct {
	from, to OriginFieldLocator
	regions  []alignmentRegion
}

func alignmentRanges(before, after string) []alignmentRegion {
	old, next := []rune(before), []rune(after)
	prefix := 0
	for prefix < len(old) && prefix < len(next) && old[prefix] == next[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(next)-prefix && old[len(old)-1-suffix] == next[len(next)-1-suffix] {
		suffix++
	}
	var regions []alignmentRegion
	if prefix > 0 {
		regions = append(regions, alignmentRegion{oldEnd: prefix, newEnd: prefix})
	}
	if suffix > 0 {
		regions = append(regions, alignmentRegion{oldStart: len(old) - suffix, oldEnd: len(old), newStart: len(next) - suffix, newEnd: len(next)})
	}
	if punctuation := alignmentPunctuationRanges(old, next); len(punctuation) > 0 {
		return punctuation
	}
	return regions
}

// Only exact substantive scalar equality permits punctuation framing to move
// an unchanged quote. This is not word similarity or a semantic classifier.
func alignmentPunctuationRanges(old, next []rune) []alignmentRegion {
	framing := func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) }
	substantive := func(values []rune) []rune {
		var result []rune
		for _, r := range values {
			if !framing(r) {
				result = append(result, r)
			}
		}
		return result
	}
	if !slices.Equal(substantive(old), substantive(next)) {
		return nil
	}
	var runs []alignmentRegion
	i, j := 0, 0
	for i < len(old) && j < len(next) {
		if old[i] != next[j] {
			if framing(next[j]) {
				j++
				continue
			}
			if framing(old[i]) {
				i++
				continue
			}
			return nil
		}
		startOld, startNew := i, j
		for i < len(old) && j < len(next) && old[i] == next[j] {
			i++
			j++
		}
		runs = append(runs, alignmentRegion{oldStart: startOld, oldEnd: i, newStart: startNew, newEnd: j})
	}
	return runs
}

func alignmentReadableFields(content PostContent) []OriginFieldLocator {
	fields := []OriginFieldLocator{{Kind: OriginFieldTitle}, {Kind: OriginFieldSummary}}
	for i := range content.Tags {
		index := i
		fields = append(fields, OriginFieldLocator{Kind: OriginFieldTag, TagIndex: &index})
	}
	for i, block := range content.Blocks {
		index := i
		switch block.Type {
		case BlockText, BlockHeading, BlockQuote:
			fields = append(fields, OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: &index})
		case BlockList:
			for j := range block.Items {
				item := j
				fields = append(fields, OriginFieldLocator{Kind: OriginFieldBlockItem, BlockIndex: &index, ItemIndex: &item})
			}
		case BlockImage, BlockGallery, BlockVideo:
			fields = append(fields, OriginFieldLocator{Kind: OriginFieldBlockAlt, BlockIndex: &index}, OriginFieldLocator{Kind: OriginFieldBlockCaption, BlockIndex: &index})
		}
	}
	return fields
}

func alignmentBlockKey(block Block) string {
	if len(block.Items) == 0 {
		block.Items = nil
	}
	if len(block.Files) == 0 {
		block.Files = nil
	}
	raw, _ := json.Marshal(block)
	return string(raw)
}

func alignmentSequencePairs(before, after []string, compatible func(int, int) bool) (map[int]int, map[int]bool) {
	old, next := map[string][]int{}, map[string][]int{}
	for i, key := range before {
		old[key] = append(old[key], i)
	}
	for i, key := range after {
		next[key] = append(next[key], i)
	}
	pairs, occupied := map[int]int{}, map[int]bool{}
	for key, positions := range old {
		if len(positions) == 1 && len(next[key]) == 1 {
			pairs[positions[0]] = next[key][0]
			occupied[next[key][0]] = true
		}
	}
	var oldRemaining, newRemaining []int
	for i := range before {
		if _, known := pairs[i]; !known {
			oldRemaining = append(oldRemaining, i)
		}
	}
	for i := range after {
		if !occupied[i] {
			newRemaining = append(newRemaining, i)
		}
	}
	if len(oldRemaining) == 1 && len(newRemaining) == 1 && compatible(oldRemaining[0], newRemaining[0]) {
		pairs[oldRemaining[0]], occupied[newRemaining[0]] = newRemaining[0], true
	}
	// A single compatible typed slot is also proven when every other new
	// slot has a different type/attachment identity (for example an edited TEXT
	// plus an inserted QUOTE). Shared-type competitors remain ambiguous.
	for _, oldIndex := range oldRemaining {
		if _, known := pairs[oldIndex]; known {
			continue
		}
		target := -1
		count := 0
		for _, newIndex := range newRemaining {
			if !occupied[newIndex] && compatible(oldIndex, newIndex) {
				target = newIndex
				count++
			}
		}
		if count != 1 {
			continue
		}
		reverse := 0
		for _, other := range oldRemaining {
			if _, known := pairs[other]; !known && compatible(other, target) {
				reverse++
			}
		}
		if reverse == 1 {
			pairs[oldIndex], occupied[target] = target, true
		}
	}
	var anchors []int
	for i := range pairs {
		anchors = append(anchors, i)
	}
	sort.Ints(anchors)
	monotonic, previous := true, -1
	for _, i := range anchors {
		if pairs[i] <= previous {
			monotonic = false
		}
		previous = pairs[i]
	}
	if monotonic {
		oldStart, newStart := -1, -1
		for _, oldEnd := range append(anchors, len(before)) {
			newEnd := len(after)
			if oldEnd < len(before) {
				newEnd = pairs[oldEnd]
			}
			if oldEnd-oldStart == 2 && newEnd-newStart == 2 && compatible(oldStart+1, newStart+1) {
				pairs[oldStart+1], occupied[newStart+1] = newStart+1, true
			}
			oldStart, newStart = oldEnd, newEnd
		}
	}
	unpaired := false
	for i := range before {
		if _, known := pairs[i]; !known {
			unpaired = true
		}
	}
	ambiguous := map[int]bool{}
	if unpaired {
		for i := range after {
			if !occupied[i] {
				ambiguous[i] = true
			}
		}
	}
	return pairs, ambiguous
}

func alignmentFieldPairs(before, after PostContent) (map[originFieldKey]alignmentFieldPair, map[originFieldKey]bool) {
	pairs, ambiguous := map[originFieldKey]alignmentFieldPair{}, map[originFieldKey]bool{}
	// Exact canonical no-op is stronger than matching individual duplicate
	// values: their existing field positions and review states remain authoritative.
	if ContentOriginIdentity(before, 0).ContentHash == ContentOriginIdentity(after, 0).ContentHash {
		for _, field := range alignmentReadableFields(before) {
			old, ok := OriginFieldText(before, field)
			next, exists := OriginFieldText(after, field)
			if ok && exists {
				pairs[originKey(field)] = alignmentFieldPair{alignmentCloneField(field), alignmentCloneField(field), alignmentRanges(old, next)}
			}
		}
		return pairs, ambiguous
	}
	add := func(from, to OriginFieldLocator) {
		old, valid := OriginFieldText(before, from)
		next, exists := OriginFieldText(after, to)
		if valid && exists {
			pairs[originKey(from)] = alignmentFieldPair{alignmentCloneField(from), alignmentCloneField(to), alignmentRanges(old, next)}
		}
	}
	for _, kind := range []OriginFieldKind{OriginFieldTitle, OriginFieldSummary} {
		field := OriginFieldLocator{Kind: kind}
		add(field, field)
	}
	tags, unknownTags := alignmentSequencePairs(before.Tags, after.Tags, func(int, int) bool { return true })
	for old, next := range tags {
		add(OriginFieldLocator{Kind: OriginFieldTag, TagIndex: alignmentIndex(old)}, OriginFieldLocator{Kind: OriginFieldTag, TagIndex: alignmentIndex(next)})
	}
	for i := range unknownTags {
		ambiguous[originKey(OriginFieldLocator{Kind: OriginFieldTag, TagIndex: alignmentIndex(i)})] = true
	}
	oldKeys, newKeys := make([]string, len(before.Blocks)), make([]string, len(after.Blocks))
	for i, block := range before.Blocks {
		oldKeys[i] = alignmentBlockKey(block)
	}
	for i, block := range after.Blocks {
		newKeys[i] = alignmentBlockKey(block)
	}
	blocks, unknownBlocks := alignmentSequencePairs(oldKeys, newKeys, func(old, next int) bool {
		left, right := before.Blocks[old], after.Blocks[next]
		return left.Type == right.Type && left.File == right.File && slices.Equal(left.Files, right.Files)
	})
	for old, next := range blocks {
		left, right := before.Blocks[old], after.Blocks[next]
		from := func(kind OriginFieldKind) OriginFieldLocator {
			return OriginFieldLocator{Kind: kind, BlockIndex: alignmentIndex(old)}
		}
		to := func(kind OriginFieldKind) OriginFieldLocator {
			return OriginFieldLocator{Kind: kind, BlockIndex: alignmentIndex(next)}
		}
		switch left.Type {
		case BlockText, BlockHeading, BlockQuote:
			add(from(OriginFieldBlockContent), to(OriginFieldBlockContent))
		case BlockImage, BlockGallery, BlockVideo:
			add(from(OriginFieldBlockAlt), to(OriginFieldBlockAlt))
			add(from(OriginFieldBlockCaption), to(OriginFieldBlockCaption))
		case BlockList:
			items, unknownItems := alignmentSequencePairs(left.Items, right.Items, func(int, int) bool { return true })
			for oldItem, nextItem := range items {
				first, second := from(OriginFieldBlockItem), to(OriginFieldBlockItem)
				first.ItemIndex, second.ItemIndex = alignmentIndex(oldItem), alignmentIndex(nextItem)
				add(first, second)
			}
			for item := range unknownItems {
				field := to(OriginFieldBlockItem)
				field.ItemIndex = alignmentIndex(item)
				ambiguous[originKey(field)] = true
			}
		}
	}
	for _, field := range alignmentReadableFields(after) {
		if field.BlockIndex != nil && unknownBlocks[*field.BlockIndex] {
			ambiguous[originKey(field)] = true
		}
	}
	return pairs, ambiguous
}

func alignmentIndex(value int) *int { return &value }

// AlignManualPlanOrigins uses the same quote/range validator with plan-local
// identity. File sets are part of paragraph correspondence; whole-plan approval
// flags grant no new semantic authorship.
func AlignManualPlanOrigins(before, after []StorylineParagraph, prior *PlanOriginReview, current, next OriginResultIdentity, ownerInputID string, attachedFilenames []string) *PlanOriginReview {
	project := func(values []StorylineParagraph) PostContent {
		result := PostContent{}
		for _, paragraph := range values {
			result.Blocks = append(result.Blocks, Block{Type: BlockText, Content: paragraph.Text, Files: slices.Clone(paragraph.Files)})
		}
		return result
	}
	var review *OriginReview
	if prior != nil {
		review = &OriginReview{Version: prior.Version, Result: prior.Result, Sources: slices.Clone(prior.Sources)}
		for _, span := range prior.Spans {
			review.Spans = append(review.Spans, OriginSpan{Field: OriginFieldLocator{Kind: OriginFieldBlockContent, BlockIndex: alignmentIndex(span.ParagraphIndex)}, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
		}
	}
	aligned := AlignManualOriginReview(project(before), project(after), review, current, next, ownerInputID, attachedFilenames).Review
	result := &PlanOriginReview{Version: aligned.Version, Result: aligned.Result, Sources: aligned.Sources}
	for _, span := range aligned.Spans {
		result.Spans = append(result.Spans, PlanOriginSpan{ParagraphIndex: *span.Field.BlockIndex, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
	}
	return result
}
