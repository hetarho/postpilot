package store

import (
	"database/sql"
	"encoding/json"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/post"
)

type originFieldJSON struct {
	Kind           string `json:"kind"`
	TagIndex       *int   `json:"tag_index,omitempty"`
	BlockIndex     *int   `json:"block_index,omitempty"`
	ItemIndex      *int   `json:"item_index,omitempty"`
	ParagraphIndex *int   `json:"paragraph_index,omitempty"`
}

type originResultJSON struct {
	ContentRevision int64  `json:"content_revision"`
	ContentHash     string `json:"content_hash"`
}

type originSourceJSON struct {
	ID                 string                `json:"id"`
	Kind               post.OriginSourceKind `json:"kind"`
	Text               string                `json:"text"`
	AttachmentFilename string                `json:"attachment_filename,omitempty"`
	AttachmentID       string                `json:"attachment_id,omitempty"`
	Available          bool                  `json:"available"`
}

type originSpanJSON struct {
	Field       originFieldJSON        `json:"field"`
	Start       int                    `json:"start"`
	End         int                    `json:"end"`
	Quote       string                 `json:"quote"`
	Category    post.OriginCategory    `json:"category"`
	SourceRefs  []string               `json:"source_refs"`
	ReviewState post.OriginReviewState `json:"review_state"`
}

type originReviewJSON struct {
	Version int                `json:"version"`
	Result  originResultJSON   `json:"result"`
	Sources []originSourceJSON `json:"sources"`
	Spans   []originSpanJSON   `json:"spans"`
}

type planOriginSpanJSON struct {
	ParagraphIndex int                    `json:"paragraph_index"`
	Start          int                    `json:"start"`
	End            int                    `json:"end"`
	Quote          string                 `json:"quote"`
	Category       post.OriginCategory    `json:"category"`
	SourceRefs     []string               `json:"source_refs"`
	ReviewState    post.OriginReviewState `json:"review_state"`
}

type planOriginReviewJSON struct {
	Version int                  `json:"version"`
	Result  originResultJSON     `json:"result"`
	Sources []originSourceJSON   `json:"sources"`
	Spans   []planOriginSpanJSON `json:"spans"`
}

type observationOriginSpanJSON struct {
	Field       string                 `json:"field"`
	ItemIndex   *int                   `json:"item_index,omitempty"`
	Start       int                    `json:"start"`
	End         int                    `json:"end"`
	Quote       string                 `json:"quote"`
	Category    post.OriginCategory    `json:"category"`
	SourceRefs  []string               `json:"source_refs"`
	ReviewState post.OriginReviewState `json:"review_state"`
}

type observationOriginReviewJSON struct {
	Version int                         `json:"version"`
	Result  originResultJSON            `json:"result"`
	Sources []originSourceJSON          `json:"sources"`
	Spans   []observationOriginSpanJSON `json:"spans"`
}

// Sidecars are optional. A valid canonical envelope is not rejected merely
// because an old/faulty annotation has the wrong JSON shape.
func (v *originReviewJSON) UnmarshalJSON(raw []byte) error {
	type wire originReviewJSON
	var decoded wire
	if json.Unmarshal(raw, &decoded) != nil {
		*v = originReviewJSON{}
		return nil
	}
	*v = originReviewJSON(decoded)
	return nil
}

func (v *planOriginReviewJSON) UnmarshalJSON(raw []byte) error {
	type wire planOriginReviewJSON
	var decoded wire
	if json.Unmarshal(raw, &decoded) != nil {
		*v = planOriginReviewJSON{}
		return nil
	}
	*v = planOriginReviewJSON(decoded)
	return nil
}

func (v *observationOriginReviewJSON) UnmarshalJSON(raw []byte) error {
	type wire observationOriginReviewJSON
	var decoded wire
	if json.Unmarshal(raw, &decoded) != nil {
		*v = observationOriginReviewJSON{}
		return nil
	}
	*v = observationOriginReviewJSON(decoded)
	return nil
}

func encodeOriginSources(sources []post.OriginSource) []originSourceJSON {
	if sources == nil {
		return nil
	}
	result := make([]originSourceJSON, len(sources))
	for i, source := range sources {
		result[i] = originSourceJSON{ID: source.ID, Kind: source.Kind, Text: source.Text, AttachmentFilename: source.AttachmentFilename, AttachmentID: source.AttachmentID, Available: source.Available}
	}
	return result
}

func decodeOriginSources(sources []originSourceJSON) []post.OriginSource {
	if sources == nil || len(sources) > post.OriginMaxSources {
		return nil
	}
	result := make([]post.OriginSource, len(sources))
	seen := make(map[string]bool, len(sources))
	for i, source := range sources {
		if source.ID == "" || !utf8.ValidString(source.ID) || utf8.RuneCountInString(source.ID) > post.OriginMaxSourceIDChars || !source.Kind.Valid() || !utf8.ValidString(source.Text) || utf8.RuneCountInString(source.Text) > post.OriginMaxSourceTextChars || seen[source.ID] {
			return nil
		}
		seen[source.ID] = true
		result[i] = post.OriginSource{ID: source.ID, Kind: source.Kind, Text: source.Text, AttachmentFilename: source.AttachmentFilename, AttachmentID: source.AttachmentID, Available: source.Available}
	}
	return result
}

func encodeOriginReview(review *post.OriginReview) *originReviewJSON {
	if review == nil {
		return nil
	}
	result := &originReviewJSON{Version: review.Version, Result: originResultJSON{ContentRevision: review.Result.ContentRevision, ContentHash: review.Result.ContentHash}, Sources: encodeOriginSources(review.Sources)}
	if review.Spans != nil {
		result.Spans = make([]originSpanJSON, len(review.Spans))
	}
	for i, span := range review.Spans {
		result.Spans[i] = originSpanJSON{Field: encodeOriginField(span.Field), Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: append([]string(nil), span.SourceRefs...), ReviewState: span.ReviewState}
	}
	return result
}

func decodeOriginReview(wire *originReviewJSON) *post.OriginReview {
	if wire == nil || wire.Version != post.OriginVersion || wire.Result.ContentHash == "" || wire.Result.ContentRevision < 0 {
		return nil
	}
	sources := decodeOriginSources(wire.Sources)
	if len(wire.Sources) != len(sources) {
		return nil
	}
	result := &post.OriginReview{Version: wire.Version, Result: post.OriginResultIdentity{ContentRevision: wire.Result.ContentRevision, ContentHash: wire.Result.ContentHash}, Sources: sources}
	if wire.Spans != nil {
		result.Spans = make([]post.OriginSpan, len(wire.Spans))
	}
	for i, span := range wire.Spans {
		if !originSpanShape(span.Start, span.End, span.Quote, span.Category, span.SourceRefs, span.ReviewState) || span.Field.ParagraphIndex != nil {
			return nil
		}
		result.Spans[i] = post.OriginSpan{Field: decodeOriginField(span.Field), Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: append([]string(nil), span.SourceRefs...), ReviewState: span.ReviewState}
	}
	return result
}

func encodePlanOrigins(review *post.PlanOriginReview) *planOriginReviewJSON {
	if review == nil {
		return nil
	}
	result := &planOriginReviewJSON{Version: review.Version, Result: originResultJSON{ContentRevision: review.Result.ContentRevision, ContentHash: review.Result.ContentHash}, Sources: encodeOriginSources(review.Sources)}
	if review.Spans != nil {
		result.Spans = make([]planOriginSpanJSON, len(review.Spans))
	}
	for i, span := range review.Spans {
		result.Spans[i] = planOriginSpanJSON{ParagraphIndex: span.ParagraphIndex, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: append([]string(nil), span.SourceRefs...), ReviewState: span.ReviewState}
	}
	return result
}

func decodePlanOrigins(wire *planOriginReviewJSON) *post.PlanOriginReview {
	if wire == nil || wire.Version != post.OriginVersion || wire.Result.ContentHash == "" || wire.Result.ContentRevision < 0 {
		return nil
	}
	sources := decodeOriginSources(wire.Sources)
	if len(wire.Sources) != len(sources) {
		return nil
	}
	result := &post.PlanOriginReview{Version: wire.Version, Result: post.OriginResultIdentity{ContentRevision: wire.Result.ContentRevision, ContentHash: wire.Result.ContentHash}, Sources: sources}
	if wire.Spans != nil {
		result.Spans = make([]post.PlanOriginSpan, len(wire.Spans))
	}
	for i, span := range wire.Spans {
		if span.ParagraphIndex < 0 || !originSpanShape(span.Start, span.End, span.Quote, span.Category, span.SourceRefs, span.ReviewState) {
			return nil
		}
		result.Spans[i] = post.PlanOriginSpan{ParagraphIndex: span.ParagraphIndex, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: append([]string(nil), span.SourceRefs...), ReviewState: span.ReviewState}
	}
	return result
}

func originSpanShape(start, end int, quote string, category post.OriginCategory, refs []string, review post.OriginReviewState) bool {
	return start >= 0 && end > start && quote != "" && utf8.ValidString(quote) && category.Valid() && review.Valid() && len(refs) <= post.OriginMaxRefsPerSpan
}

func encodeObservationOrigins(review *post.ObservationOriginReview) *observationOriginReviewJSON {
	if review == nil {
		return nil
	}
	result := &observationOriginReviewJSON{Version: review.Version, Result: originResultJSON{ContentRevision: review.Result.ContentRevision, ContentHash: review.Result.ContentHash}, Sources: encodeOriginSources(review.Sources)}
	if review.Spans != nil {
		result.Spans = make([]observationOriginSpanJSON, len(review.Spans))
	}
	for i, span := range review.Spans {
		result.Spans[i] = observationOriginSpanJSON{Field: span.Field, ItemIndex: copyOriginIndex(span.ItemIndex), Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: append([]string(nil), span.SourceRefs...), ReviewState: span.ReviewState}
	}
	return result
}

func decodeObservationOrigins(wire *observationOriginReviewJSON) *post.ObservationOriginReview {
	if wire == nil || wire.Version != post.OriginVersion || wire.Result.ContentHash == "" || wire.Result.ContentRevision < 0 {
		return nil
	}
	sources := decodeOriginSources(wire.Sources)
	if len(wire.Sources) != len(sources) {
		return nil
	}
	result := &post.ObservationOriginReview{Version: wire.Version, Result: post.OriginResultIdentity{ContentRevision: wire.Result.ContentRevision, ContentHash: wire.Result.ContentHash}, Sources: sources}
	if wire.Spans != nil {
		result.Spans = make([]post.ObservationOriginSpan, len(wire.Spans))
	}
	for i, span := range wire.Spans {
		if span.Field == "" || !originSpanShape(span.Start, span.End, span.Quote, span.Category, span.SourceRefs, span.ReviewState) {
			return nil
		}
		result.Spans[i] = post.ObservationOriginSpan{Field: span.Field, ItemIndex: copyOriginIndex(span.ItemIndex), Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: append([]string(nil), span.SourceRefs...), ReviewState: span.ReviewState}
	}
	return result
}

func encodeOriginField(field post.OriginFieldLocator) originFieldJSON {
	return originFieldJSON{Kind: string(field.Kind), TagIndex: copyOriginIndex(field.TagIndex), BlockIndex: copyOriginIndex(field.BlockIndex), ItemIndex: copyOriginIndex(field.ItemIndex)}
}

func decodeOriginField(field originFieldJSON) post.OriginFieldLocator {
	return post.OriginFieldLocator{Kind: post.OriginFieldKind(field.Kind), TagIndex: copyOriginIndex(field.TagIndex), BlockIndex: copyOriginIndex(field.BlockIndex), ItemIndex: copyOriginIndex(field.ItemIndex)}
}

func copyOriginIndex(index *int) *int {
	if index == nil {
		return nil
	}
	copy := *index
	return &copy
}

func cloneOriginReview(review *post.OriginReview) *post.OriginReview {
	if review == nil {
		return nil
	}
	copy := *review
	copy.Sources = append([]post.OriginSource(nil), review.Sources...)
	copy.Spans = append([]post.OriginSpan(nil), review.Spans...)
	for i, span := range copy.Spans {
		copy.Spans[i].Field = decodeOriginField(encodeOriginField(span.Field))
		copy.Spans[i].SourceRefs = append([]string(nil), span.SourceRefs...)
	}
	return &copy
}

func clonePlanOrigins(review *post.PlanOriginReview) *post.PlanOriginReview {
	if review == nil {
		return nil
	}
	copy := *review
	copy.Sources = append([]post.OriginSource(nil), review.Sources...)
	copy.Spans = append([]post.PlanOriginSpan(nil), review.Spans...)
	for i, span := range copy.Spans {
		copy.Spans[i].SourceRefs = append([]string(nil), span.SourceRefs...)
	}
	return &copy
}

func cloneObservationOrigins(review *post.ObservationOriginReview) *post.ObservationOriginReview {
	if review == nil {
		return nil
	}
	copy := *review
	copy.Sources = append([]post.OriginSource(nil), review.Sources...)
	copy.Spans = append([]post.ObservationOriginSpan(nil), review.Spans...)
	for i, span := range copy.Spans {
		copy.Spans[i].ItemIndex = copyOriginIndex(span.ItemIndex)
		copy.Spans[i].SourceRefs = append([]string(nil), span.SourceRefs...)
	}
	return &copy
}

func marshalOriginReview(review *post.OriginReview) sql.NullString {
	if review == nil {
		return sql.NullString{}
	}
	raw, err := json.Marshal(encodeOriginReview(review))
	if err != nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}
func unmarshalOriginReview(value sql.NullString) *post.OriginReview {
	if !value.Valid {
		return nil
	}
	var wire originReviewJSON
	if json.Unmarshal([]byte(value.String), &wire) != nil {
		return nil
	}
	return decodeOriginReview(&wire)
}
func marshalPlanOrigins(review *post.PlanOriginReview) sql.NullString {
	if review == nil {
		return sql.NullString{}
	}
	raw, err := json.Marshal(encodePlanOrigins(review))
	if err != nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}
func unmarshalPlanOrigins(value sql.NullString) *post.PlanOriginReview {
	if !value.Valid {
		return nil
	}
	var wire planOriginReviewJSON
	if json.Unmarshal([]byte(value.String), &wire) != nil {
		return nil
	}
	return decodePlanOrigins(&wire)
}

func unmarshalPlanSources(value sql.NullString) []post.OriginSource {
	review := unmarshalPlanOrigins(value)
	if review == nil {
		return nil
	}
	return review.Sources
}
