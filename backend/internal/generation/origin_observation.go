package generation

import (
	"slices"

	"github.com/postpilot/backend/internal/post"
)

// Observation-origin fields describe the supplied photo/video interpretations,
// never owner authorship. The public locator remains an observation field; the
// post projection below is only a local reuse of its exact Unicode quote checks.
func observationOriginContent(observation Observation) PostContent {
	content := PostContent{Blocks: []Block{{Type: BlockText, Content: observation.Scene}, {Type: BlockText, Content: observation.Mood},
		{Type: BlockText, Content: observation.VisibleText}, {Type: BlockText, Content: observation.Speech}}}
	for _, object := range observation.Objects {
		content.Blocks = append(content.Blocks, Block{Type: BlockText, Content: object})
	}
	for _, event := range observation.Events {
		content.Blocks = append(content.Blocks, Block{Type: BlockText, Content: event})
	}
	return content
}

func observationOriginIndex(observation Observation, field string, item *int) int {
	if item == nil {
		switch field {
		case "scene":
			return 0
		case "mood":
			return 1
		case "visible_text":
			return 2
		case "speech":
			return 3
		}
		return -1
	}
	if *item < 0 {
		return -1
	}
	switch field {
	case "object":
		if *item < len(observation.Objects) {
			return 4 + *item
		}
	case "event":
		if *item < len(observation.Events) {
			return 4 + len(observation.Objects) + *item
		}
	}
	return -1
}

func observationOriginField(observation Observation, index int) (string, *int) {
	switch index {
	case 0:
		return "scene", nil
	case 1:
		return "mood", nil
	case 2:
		return "visible_text", nil
	case 3:
		return "speech", nil
	}
	if index < 4+len(observation.Objects) {
		item := index - 4
		return "object", &item
	}
	item := index - 4 - len(observation.Objects)
	return "event", &item
}

// ValidateObservationOrigins validates only optional annotations. Missing or
// legacy sidecars yield no guessed visual spans, while canonical observation
// members remain untouched. Actual video delivery must be supplied to establish
// observed source-time event/speech origins; a photo has neither contract.
func ValidateObservationOrigins(observation Observation, sources []post.OriginSource, kind ...AttachmentKind) *ObservationOriginReview {
	video := len(kind) == 1 && kind[0] == AttachmentVideo
	allowedSources := slices.Clone(sources)
	for i := range allowedSources {
		if allowedSources[i].Kind != post.OriginSourceVisualObservation || allowedSources[i].AttachmentFilename != observation.File {
			allowedSources[i].Available = false
		}
	}
	candidates := make([]post.OriginCandidate, len(observation.OriginCandidates))
	for i, candidate := range observation.OriginCandidates {
		index := observationOriginIndex(observation, candidate.Field, candidate.ItemIndex)
		if candidate.Category == post.OriginOwnerInput || (!video && candidate.Category == post.OriginPhotoInterpretation && (candidate.Field == "event" || candidate.Field == "speech")) {
			index = -1
		}
		candidates[i] = post.OriginCandidate{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index},
			Quote: candidate.Quote, Occurrence: cloneOriginIndex(candidate.Occurrence), Category: candidate.Category, SourceRefs: slices.Clone(candidate.SourceRefs)}
	}
	resolved := ResolveWriteOriginCandidates(observationOriginContent(observation), allowedSources, candidates)

	result := &ObservationOriginReview{Version: post.OriginVersion, Result: ObservationOriginIdentity(observation), Sources: resolved.Review.Sources}
	for _, span := range resolved.Review.Spans {
		field, item := observationOriginField(observation, *span.Field.BlockIndex)
		result.Spans = append(result.Spans, ObservationOriginSpan{Field: field, ItemIndex: item, Start: span.Start, End: span.End,
			Quote: span.Quote, Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
	}
	return result
}

func ObservationOriginIdentity(observation Observation) post.OriginResultIdentity {
	identity := struct {
		File, Scene, Mood, VisibleText, Speech string
		Objects, Events                        []string
		PeoplePresent                          bool
	}{observation.File, observation.Scene, observation.Mood, observation.VisibleText, observation.Speech, observation.Objects, observation.Events, observation.PeoplePresent}
	return post.OriginResultIdentity{ContentHash: originHash(identity)}
}

// ValidateStoredObservationOrigins verifies an actually captured result before
// its phrases become writing material. Missing/stale data is not reconstructed.
func ValidateStoredObservationOrigins(observation Observation, review *ObservationOriginReview, kind ...AttachmentKind) *ObservationOriginReview {
	identity := ObservationOriginIdentity(observation)
	result := &ObservationOriginReview{Version: post.OriginVersion, Result: identity}
	if review == nil || review.Version != post.OriginVersion || review.Result != identity {
		return result
	}
	video := len(kind) == 1 && kind[0] == AttachmentVideo
	projected := post.OriginReview{Version: review.Version, Result: identity, Sources: slices.Clone(review.Sources)}
	for _, span := range review.Spans {
		index := observationOriginIndex(observation, span.Field, span.ItemIndex)
		if span.Category == post.OriginOwnerInput || (!video && span.Category == post.OriginPhotoInterpretation && (span.Field == "event" || span.Field == "speech")) {
			index = -1
		}
		projected.Spans = append(projected.Spans, post.OriginSpan{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index}, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
	}
	validated := post.ValidateOriginReview(originPostContent(observationOriginContent(observation)), identity, &projected)
	catalog := map[string]post.OriginSource{}
	for _, source := range validated.Review.Sources {
		catalog[source.ID] = source
	}
	result.Sources = validated.Review.Sources
	for _, span := range validated.Review.Spans {
		if !originSourceCategoryCompatible(span.Category, span.SourceRefs, catalog) {
			continue
		}
		sameMedia := true
		for _, ref := range span.SourceRefs {
			source := catalog[ref]
			if source.Kind != post.OriginSourceVisualObservation || source.AttachmentFilename != observation.File {
				sameMedia = false
			}
		}
		if !sameMedia {
			continue
		}
		field, item := observationOriginField(observation, *span.Field.BlockIndex)
		result.Spans = append(result.Spans, ObservationOriginSpan{Field: field, ItemIndex: item, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: slices.Clone(span.SourceRefs), ReviewState: span.ReviewState})
	}
	return result
}
