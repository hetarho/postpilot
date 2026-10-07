package generation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"sort"

	"github.com/postpilot/backend/internal/post"
)

const (
	OriginIssueSourceCategory post.OriginIssueCode = "source_category_mismatch"
	OriginIssueNormalization  post.OriginIssueCode = "normalization_unproved"
)

// OriginContentIdentity identifies the exact stage result. The publishing owner
// assigns its durable revision/hash; this staging identity never invents one.
func OriginContentIdentity(content PostContent) post.OriginResultIdentity {
	return post.OriginResultIdentity{ContentHash: originHash(originPostContent(content))}
}

func originHash(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func originPostContent(content PostContent) post.PostContent {
	result := post.PostContent{Title: content.Title, Summary: content.Summary, Tags: slices.Clone(content.Tags)}
	for _, block := range content.Blocks {
		result.Blocks = append(result.Blocks, post.Block{Type: post.BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: slices.Clone(block.Items), Files: slices.Clone(block.Files), Layout: post.GalleryLayout(block.Layout)})
	}
	return result
}

// ResolveWriteOriginCandidates checks exact current text with the shared post
// resolver, then fences source categories. Neither layer certifies semantic
// support: a compatible memo ref is not proof of every claim in a paraphrase.
func ResolveWriteOriginCandidates(content PostContent, sources []post.OriginSource, candidates []post.OriginCandidate) post.OriginResolution {
	resolved := post.ResolveOriginCandidates(originPostContent(content), OriginContentIdentity(content), sources, candidates)
	catalog := make(map[string]post.OriginSource, len(resolved.Review.Sources))
	for _, source := range resolved.Review.Sources {
		catalog[source.ID] = source
	}
	kept := resolved.Review.Spans[:0]
	for _, span := range resolved.Review.Spans {
		if !originSourceCategoryCompatible(span.Category, span.SourceRefs, catalog) || !originMediaFieldCompatible(content, span, catalog) {
			index := -1
			for i, candidate := range candidates {
				if originLocatorEqual(candidate.Field, span.Field) && candidate.Quote == span.Quote && candidate.Category == span.Category && slices.Equal(candidate.SourceRefs, span.SourceRefs) {
					index = i
					break
				}
			}
			resolved.Issues = append(resolved.Issues, post.OriginIssue{Index: index, Code: OriginIssueSourceCategory})
			continue
		}
		kept = append(kept, span)
	}
	resolved.Review.Spans = kept
	sort.SliceStable(resolved.Issues, func(i, j int) bool { return resolved.Issues[i].Index < resolved.Issues[j].Index })
	return resolved
}

func originMediaFieldCompatible(content PostContent, span post.OriginSpan, catalog map[string]post.OriginSource) bool {
	if span.Category != post.OriginPhotoInterpretation || (span.Field.Kind != post.OriginFieldBlockAlt && span.Field.Kind != post.OriginFieldBlockCaption) {
		return true
	}
	if span.Field.BlockIndex == nil || *span.Field.BlockIndex < 0 || *span.Field.BlockIndex >= len(content.Blocks) {
		return false
	}
	block := content.Blocks[*span.Field.BlockIndex]
	files := []string{block.File}
	if block.Type == BlockGallery {
		files = block.Files
	}
	for _, ref := range span.SourceRefs {
		if !slices.Contains(files, catalog[ref].AttachmentFilename) {
			return false
		}
	}
	return true
}

func originSourceCategoryCompatible(category post.OriginCategory, refs []string, catalog map[string]post.OriginSource) bool {
	for _, ref := range refs {
		source, ok := catalog[ref]
		if !ok || !source.Available {
			return false
		}
		switch category {
		case post.OriginOwnerInput:
			switch source.Kind {
			case post.OriginSourceMemo, post.OriginSourceTemplateAnswer, post.OriginSourceOwnerEdit, post.OriginSourceMemory, post.OriginSourceLiteralText:
			default:
				return false
			}
		case post.OriginPhotoInterpretation:
			if source.Kind != post.OriginSourceVisualObservation || source.AttachmentFilename == "" {
				return false
			}
		case post.OriginAIAdded:
			// A proposal may explain its actual supplied context without promoting
			// that context into owner facts or visual evidence.
		default:
			return false
		}
	}
	return category == post.OriginAIAdded || len(refs) > 0
}

// FinalizeWriteOrigins performs the existing canonical normalization with an
// explicit field map. An invalid/missing sidecar cannot alter usable content.
func FinalizeWriteOrigins(answer WriteAnswer, photos, videos []string, portrait map[string]bool, sources []post.OriginSource) WriteAnswer {
	before := answer.Content
	after, mapping := NormalizeContentWithOriginMap(before, photos, videos, portrait)
	candidates, _ := RemapOriginCandidates(before, after, mapping, answer.OriginCandidates)
	sources = OriginSourcesWithAttachments(sources, photos, videos)
	resolved := ResolveWriteOriginCandidates(after, sources, candidates)
	answer.Content = after
	answer.OriginCandidates = candidates
	answer.Origins = &resolved.Review
	if answer.Storyline != nil {
		storyline := *answer.Storyline
		review := ValidatePlanOrigins(storyline.Paragraphs, sources, storyline.OriginCandidates)
		storyline.Origins = &review
		answer.Storyline = &storyline
	}
	return answer
}

// OriginSourcesWithAttachments fences frozen visual evidence against the actual
// attachment snapshot without looking up or reconstructing any source.
func OriginSourcesWithAttachments(sources []post.OriginSource, photos, videos []string) []post.OriginSource {
	result := slices.Clone(sources)
	attached := nameSet(append(append([]string(nil), photos...), videos...))
	for i := range result {
		if result[i].Kind == post.OriginSourceVisualObservation {
			if _, exists := attached[result[i].AttachmentFilename]; !exists {
				result[i].Available = false
			}
		}
	}
	return result
}

type PlanOriginResolution struct {
	Review PlanOriginReview
	Issues []post.OriginIssue
}

func PlanOriginIdentity(paragraphs []StorylineParagraph) post.OriginResultIdentity {
	return post.OriginResultIdentity{ContentHash: originHash(paragraphs)}
}

func planOriginContent(paragraphs []StorylineParagraph) PostContent {
	content := PostContent{}
	for _, paragraph := range paragraphs {
		content.Blocks = append(content.Blocks, Block{Type: BlockText, Content: paragraph.Text})
	}
	return content
}

// ResolvePlanOriginCandidates uses a dedicated paragraph result, independent of
// whole-plan approval/EditedByHand and of the eventual canonical post result.
func ResolvePlanOriginCandidates(paragraphs []StorylineParagraph, sources []post.OriginSource, candidates []PlanOriginCandidate) PlanOriginResolution {
	mapped := make([]post.OriginCandidate, len(candidates))
	for i, candidate := range candidates {
		index := candidate.ParagraphIndex
		mapped[i] = post.OriginCandidate{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index},
			Quote: candidate.Quote, Occurrence: cloneOriginIndex(candidate.Occurrence), Category: candidate.Category, SourceRefs: slices.Clone(candidate.SourceRefs)}
	}
	resolved := ResolveWriteOriginCandidates(planOriginContent(paragraphs), sources, mapped)
	result := PlanOriginResolution{Review: PlanOriginReview{Version: post.OriginVersion, Result: PlanOriginIdentity(paragraphs), Sources: resolved.Review.Sources}, Issues: resolved.Issues}
	for _, span := range resolved.Review.Spans {
		result.Review.Spans = append(result.Review.Spans, PlanOriginSpan{ParagraphIndex: *span.Field.BlockIndex, Start: span.Start, End: span.End,
			Quote: span.Quote, Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
	}
	return result
}

func ValidatePlanOrigins(paragraphs []StorylineParagraph, sources []post.OriginSource, candidates []PlanOriginCandidate) PlanOriginReview {
	return ResolvePlanOriginCandidates(paragraphs, sources, candidates).Review
}

// PreserveRevisionOrigins retains only exact untouched fields and known original
// evidence. A later candidate cannot recolor an unchanged AI meaning. A durable
// review requires its authenticated current identity; absent identity admits only
// an exactly matching stage-local review, never guessed historical provenance.
func PreserveRevisionOrigins(current PostContent, beforeOrigins *post.OriginReview, after PostContent, newSources []post.OriginSource, newCandidates []post.OriginCandidate, currentResult ...post.OriginResultIdentity) *post.OriginReview {
	beforeContent, afterContent := originPostContent(current), originPostContent(after)
	fieldPairs, ambiguousFields := revisionOriginFieldPairs(current, after)
	ranges := map[originFieldKey][]originUnchangedRange{}
	for _, pair := range fieldPairs {
		ranges[originKey(pair.to)] = pair.regions
	}

	var prior post.OriginReview
	if beforeOrigins != nil {
		identity := OriginContentIdentity(current)
		validIdentity := true
		if len(currentResult) == 1 {
			identity = currentResult[0]
		} else if len(currentResult) > 1 || beforeOrigins.Result.ContentRevision != 0 {
			validIdentity = false
		}
		if validIdentity {
			prior = post.ValidateOriginReview(beforeContent, identity, beforeOrigins).Review
		}
	}
	catalog := map[string]post.OriginSource{}
	for _, source := range prior.Sources {
		catalog[source.ID] = source
	}
	var retained []post.OriginSpan
	for _, span := range prior.Spans {
		if !originSourceCategoryCompatible(span.Category, span.SourceRefs, catalog) {
			continue
		}
		pair, proven := fieldPairs[originKey(span.Field)]
		if !proven {
			continue
		}
		for _, region := range pair.regions {
			if span.Start >= region.OldStart && span.End <= region.OldEnd {
				next := span
				next.Field = cloneOriginLocator(pair.to)
				next.Start = region.NewStart + span.Start - region.OldStart
				next.End = next.Start + span.End - span.Start
				retained = append(retained, next)
				break
			}
		}
	}
	var candidates []post.OriginCandidate
	for _, candidate := range newCandidates {
		if ambiguousFields[originKey(candidate.Field)] {
			continue
		}
		text, ok := post.OriginFieldText(afterContent, candidate.Field)
		if !ok {
			candidates = append(candidates, candidate)
			continue
		}
		probe := post.OriginCandidate{Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Quote: candidate.Quote, Occurrence: candidate.Occurrence, Category: post.OriginAIAdded}
		resolved := post.ResolveOriginCandidates(post.PostContent{Title: text}, post.OriginResultIdentity{ContentHash: "revision-field"}, nil, []post.OriginCandidate{probe})
		reject := false
		if len(resolved.Review.Spans) == 1 {
			span := resolved.Review.Spans[0]
			for _, region := range ranges[originKey(candidate.Field)] {
				if span.Start >= region.NewStart && span.End <= region.NewEnd {
					reject = true
					break
				}
			}
			for _, known := range retained {
				if originLocatorEqual(candidate.Field, known.Field) && span.Start < known.End && span.End > known.Start {
					reject = true
					break
				}
			}
		}
		if !reject {
			candidates = append(candidates, candidate)
		}
	}
	fresh := ResolveWriteOriginCandidates(after, newSources, candidates).Review
	freshCatalog := map[string]post.OriginSource{}
	for _, source := range fresh.Sources {
		freshCatalog[source.ID] = source
	}
	for _, span := range retained {
		withdrawn := false
		for _, ref := range span.SourceRefs {
			old := catalog[ref]
			if now, exists := freshCatalog[ref]; exists && old.Kind == post.OriginSourceVisualObservation && now.Kind == old.Kind && now.AttachmentFilename == old.AttachmentFilename && !now.Available {
				withdrawn = true
			}
		}
		if withdrawn {
			continue
		}
		refs := slices.Clone(span.SourceRefs)
		for i, ref := range refs {
			if value, exists := freshCatalog[ref]; exists && value != catalog[ref] {
				source := catalog[ref]
				source.ID = "retained-" + originHash(source)
				refs[i] = source.ID
				if _, exists := freshCatalog[source.ID]; !exists {
					fresh.Sources = append(fresh.Sources, source)
					freshCatalog[source.ID] = source
				}
			}
		}
		span.SourceRefs = refs
		fresh.Spans = append(fresh.Spans, span)
		for _, ref := range span.SourceRefs {
			if _, exists := freshCatalog[ref]; !exists {
				fresh.Sources = append(fresh.Sources, catalog[ref])
				freshCatalog[ref] = catalog[ref]
			}
		}
	}
	validated := post.ValidateOriginReview(afterContent, fresh.Result, &fresh)
	return &validated.Review
}

type originUnchangedRange struct{ OldStart, OldEnd, NewStart, NewEnd int }

// Exact scalar prefix/suffix regions prove retained text without choosing among
// equal interior phrases or interpreting a stylistic/paraphrased meaning.
func unchangedOriginRanges(before, after string) []originUnchangedRange {
	old, next := []rune(before), []rune(after)
	prefix := 0
	for prefix < len(old) && prefix < len(next) && old[prefix] == next[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(next)-prefix && old[len(old)-1-suffix] == next[len(next)-1-suffix] {
		suffix++
	}
	var result []originUnchangedRange
	if prefix > 0 {
		result = append(result, originUnchangedRange{OldEnd: prefix, NewEnd: prefix})
	}
	if suffix > 0 {
		result = append(result, originUnchangedRange{OldStart: len(old) - suffix, OldEnd: len(old), NewStart: len(next) - suffix, NewEnd: len(next)})
	}
	return result
}
