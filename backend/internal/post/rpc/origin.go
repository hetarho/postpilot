package rpc

import (
	"fmt"
	"math"
	"slices"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

var originCategories = map[postpilotv1.SemanticOriginCategory]post.OriginCategory{
	postpilotv1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_UNSPECIFIED:          "",
	postpilotv1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_OWNER_INPUT:          post.OriginOwnerInput,
	postpilotv1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_PHOTO_INTERPRETATION: post.OriginPhotoInterpretation,
	postpilotv1.SemanticOriginCategory_SEMANTIC_ORIGIN_CATEGORY_AI_ADDED:             post.OriginAIAdded,
}

var originReviewStates = map[postpilotv1.OriginReviewState]post.OriginReviewState{
	postpilotv1.OriginReviewState_ORIGIN_REVIEW_STATE_UNSPECIFIED: "",
	postpilotv1.OriginReviewState_ORIGIN_REVIEW_STATE_UNCONFIRMED: post.OriginUnconfirmed,
	postpilotv1.OriginReviewState_ORIGIN_REVIEW_STATE_UNREVIEWED:  post.OriginUnreviewed,
	postpilotv1.OriginReviewState_ORIGIN_REVIEW_STATE_CONFIRMED:   post.OriginConfirmed,
}

var originFieldKinds = map[postpilotv1.OriginFieldKind]post.OriginFieldKind{
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_UNSPECIFIED:   "",
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_TITLE:         post.OriginFieldTitle,
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_SUMMARY:       post.OriginFieldSummary,
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_TAG:           post.OriginFieldTag,
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_CONTENT: post.OriginFieldBlockContent,
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_ITEM:    post.OriginFieldBlockItem,
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_ALT:     post.OriginFieldBlockAlt,
	postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_BLOCK_CAPTION: post.OriginFieldBlockCaption,
}

// OriginReviewFromProto deliberately retains invalid metadata for the separate
// domain validator. Unknown enum values remain invalid, never a guessed category.
func OriginReviewFromProto(value *postpilotv1.OriginReview) *post.OriginReview {
	if value == nil {
		return nil
	}
	review := &post.OriginReview{Version: int(value.Version), Result: post.OriginResultIdentity{
		ContentRevision: value.GetResult().GetContentRevision(), ContentHash: value.GetResult().GetContentHash(),
	}}
	for _, source := range value.Sources {
		review.Sources = append(review.Sources, post.OriginSource{ID: source.GetId(), Kind: post.OriginSourceKind(source.GetKind()),
			Text: source.GetText(), AttachmentFilename: source.GetAttachmentFilename(), AttachmentID: source.GetAttachmentId(), Available: source.GetAvailable()})
	}
	for _, span := range value.Spans {
		review.Spans = append(review.Spans, post.OriginSpan{Field: originFieldFromProto(span.GetField()),
			Start: int(span.GetStart()), End: int(span.GetEnd()), Quote: span.GetQuote(),
			Category: originCategories[span.GetCategory()], SourceRefs: slices.Clone(span.GetSourceRefs()),
			ReviewState: originReviewStates[span.GetReviewState()]})
	}
	return review
}

func originFieldFromProto(value *postpilotv1.OriginFieldLocator) post.OriginFieldLocator {
	field := post.OriginFieldLocator{Kind: originFieldKinds[value.GetKind()]}
	if value != nil {
		field.TagIndex = originIndexFromProto(value.TagIndex)
		field.BlockIndex = originIndexFromProto(value.BlockIndex)
		field.ItemIndex = originIndexFromProto(value.ItemIndex)
	}
	return field
}

func originIndexFromProto(value *uint32) *int {
	if value == nil {
		return nil
	}
	index := int(*value)
	return &index
}

func originIndexToProto(value *int) (*uint32, error) {
	if value == nil {
		return nil, nil
	}
	if *value < 0 || uint64(*value) > math.MaxUint32 {
		return nil, fmt.Errorf("origin index is outside uint32")
	}
	index := uint32(*value)
	return &index, nil
}

func originWire[T comparable, W comparable](values map[W]T, value T) (W, bool) {
	for wire, domain := range values {
		if domain == value {
			return wire, true
		}
	}
	var zero W
	return zero, false
}

// OriginReviewToProto converts validated result metadata without touching canonical
// content. It rejects unsafe integer casts and unknown closed vocabularies.
func OriginReviewToProto(review *post.OriginReview) (*postpilotv1.OriginReview, error) {
	if review == nil {
		return nil, nil
	}
	if review.Version != post.OriginVersion || review.Result.ContentRevision < 0 || review.Result.ContentHash == "" {
		return nil, fmt.Errorf("invalid origin result identity or version")
	}
	value := &postpilotv1.OriginReview{Version: uint32(review.Version), Result: &postpilotv1.OriginResultIdentity{
		ContentRevision: review.Result.ContentRevision, ContentHash: review.Result.ContentHash,
	}}
	for _, source := range review.Sources {
		value.Sources = append(value.Sources, &postpilotv1.OriginSource{Id: source.ID, Kind: string(source.Kind),
			Text: source.Text, AttachmentFilename: source.AttachmentFilename, AttachmentId: source.AttachmentID, Available: source.Available})
	}
	for _, span := range review.Spans {
		category, categoryOK := originWire(originCategories, span.Category)
		state, stateOK := originWire(originReviewStates, span.ReviewState)
		kind, kindOK := originWire(originFieldKinds, span.Field.Kind)
		if !categoryOK || !stateOK || !kindOK || !span.Category.Valid() || !span.ReviewState.Valid() || span.Field.Kind == "" ||
			span.Start < 0 || span.End <= span.Start || uint64(span.End) > math.MaxUint32 {
			return nil, fmt.Errorf("invalid origin span vocabulary or range")
		}
		tag, err := originIndexToProto(span.Field.TagIndex)
		if err != nil {
			return nil, err
		}
		block, err := originIndexToProto(span.Field.BlockIndex)
		if err != nil {
			return nil, err
		}
		item, err := originIndexToProto(span.Field.ItemIndex)
		if err != nil {
			return nil, err
		}
		value.Spans = append(value.Spans, &postpilotv1.OriginSpan{
			Field: &postpilotv1.OriginFieldLocator{Kind: kind, TagIndex: tag, BlockIndex: block, ItemIndex: item},
			Start: uint32(span.Start), End: uint32(span.End), Quote: span.Quote, Category: category,
			SourceRefs: slices.Clone(span.SourceRefs), ReviewState: state,
		})
	}
	return value, nil
}
