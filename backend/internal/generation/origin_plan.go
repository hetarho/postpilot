package generation

import (
	"slices"

	"github.com/postpilot/backend/internal/post"
)

// ValidateStoredPlanOrigins verifies an actual plan-local result, independently
// of choosing its arrangement or marking the whole plan EditedByHand.
func ValidateStoredPlanOrigins(paragraphs []StorylineParagraph, review *PlanOriginReview) *PlanOriginReview {
	identity := PlanOriginIdentity(paragraphs)
	result := &PlanOriginReview{Version: post.OriginVersion, Result: identity}
	if review == nil || review.Version != post.OriginVersion || review.Result != identity {
		return result
	}
	projected := post.OriginReview{Version: review.Version, Result: identity, Sources: slices.Clone(review.Sources)}
	for _, span := range review.Spans {
		index := span.ParagraphIndex
		projected.Spans = append(projected.Spans, post.OriginSpan{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index},
			Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
	}
	validated := post.ValidateOriginReview(originPostContent(planOriginContent(paragraphs)), identity, &projected)
	catalog := map[string]post.OriginSource{}
	for _, source := range validated.Review.Sources {
		catalog[source.ID] = source
	}
	result.Sources = validated.Review.Sources
	for _, span := range validated.Review.Spans {
		if !originSourceCategoryCompatible(span.Category, span.SourceRefs, catalog) {
			continue
		}
		result.Spans = append(result.Spans, PlanOriginSpan{ParagraphIndex: *span.Field.BlockIndex, Start: span.Start, End: span.End, Quote: span.Quote,
			Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
	}
	return result
}

func planParagraphOriginKey(paragraph StorylineParagraph) string {
	value := paragraph
	if len(value.Files) == 0 {
		value.Files = nil
	}
	return originHash(value)
}

// PreservePlanOrigins proves only unique exact paragraph text+file matches. It
// may reindex those matches when rearranged, but never chooses among duplicate
// paragraphs or treats approval/placement as new owner evidence.
func PreservePlanOrigins(before []StorylineParagraph, prior *PlanOriginReview, after []StorylineParagraph, newSources []post.OriginSource, newCandidates []PlanOriginCandidate) *PlanOriginReview {
	old := map[string][]int{}
	next := map[string][]int{}
	oldTexts := map[string]bool{}
	for i, paragraph := range before {
		key := planParagraphOriginKey(paragraph)
		old[key] = append(old[key], i)
		oldTexts[paragraph.Text] = true
	}
	for i, paragraph := range after {
		key := planParagraphOriginKey(paragraph)
		next[key] = append(next[key], i)
	}
	mapping := map[int]int{}
	for key, positions := range old {
		if len(positions) == 1 && len(next[key]) == 1 {
			mapping[positions[0]] = next[key][0]
		}
	}
	var candidates []PlanOriginCandidate
	for _, candidate := range newCandidates {
		if candidate.ParagraphIndex >= 0 && candidate.ParagraphIndex < len(after) && oldTexts[after[candidate.ParagraphIndex].Text] {
			continue
		}
		candidates = append(candidates, candidate)
	}
	fresh := ValidatePlanOrigins(after, newSources, candidates)
	validated := ValidateStoredPlanOrigins(before, prior)
	catalog := map[string]post.OriginSource{}
	for _, source := range validated.Sources {
		catalog[source.ID] = source
	}
	freshCatalog := map[string]post.OriginSource{}
	for _, source := range fresh.Sources {
		freshCatalog[source.ID] = source
	}
	for _, span := range validated.Spans {
		index, proven := mapping[span.ParagraphIndex]
		if !proven {
			continue
		}
		withdrawn := false
		for _, ref := range span.SourceRefs {
			oldSource := catalog[ref]
			if current, exists := freshCatalog[ref]; exists && oldSource.Kind == post.OriginSourceVisualObservation && current.Kind == oldSource.Kind && current.AttachmentFilename == oldSource.AttachmentFilename && !current.Available {
				withdrawn = true
			}
		}
		if withdrawn {
			continue
		}
		span.ParagraphIndex = index
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
	return ValidateStoredPlanOrigins(after, &fresh)
}
