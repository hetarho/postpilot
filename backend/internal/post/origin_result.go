package post

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"time"
)

type PlanOriginSpan struct {
	ParagraphIndex, Start, End int
	Quote                      string
	Category                   OriginCategory
	SourceRefs                 []string
	ReviewState                OriginReviewState
}

type PlanOriginReview struct {
	Version int
	Result  OriginResultIdentity
	Sources []OriginSource
	Spans   []PlanOriginSpan
}

type ObservationOriginSpan struct {
	Field       string
	ItemIndex   *int
	Start, End  int
	Quote       string
	Category    OriginCategory
	SourceRefs  []string
	ReviewState OriginReviewState
}

type ObservationOriginReview struct {
	Version int
	Result  OriginResultIdentity
	Sources []OriginSource
	Spans   []ObservationOriginSpan
}

// ContentOriginIdentity uses the stable domain field shape while normalizing
// empty collections, which the existing storage codec spells as omitted fields.
// Presentation sidecars and machine-baseline fields never enter this hash.
func ContentOriginIdentity(content PostContent, revision int64) OriginResultIdentity {
	copy := content
	copy.Tags = originOrNil(copy.Tags)
	copy.Blocks = originOrNil(copy.Blocks)
	for i := range copy.Blocks {
		copy.Blocks[i].Items = originOrNil(copy.Blocks[i].Items)
		copy.Blocks[i].Files = originOrNil(copy.Blocks[i].Files)
	}
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(raw)
	return OriginResultIdentity{ContentRevision: revision, ContentHash: hex.EncodeToString(sum[:])}
}

func PlanOriginIdentity(paragraphs []StorylineParagraph) OriginResultIdentity {
	copy := slices.Clone(paragraphs)
	for i := range copy {
		copy[i].Files = originOrNil(copy[i].Files)
	}
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(raw)
	return OriginResultIdentity{ContentHash: hex.EncodeToString(sum[:])}
}

// Observation identity covers its actual material, excluding presentation,
// model provenance and rotation. Empty arrays have the durable omitted shape.
func ObservationOriginIdentity(observation Observation) OriginResultIdentity {
	value := struct {
		File, Scene, Mood, VisibleText, Speech string
		Objects, Events                        []string
		PeoplePresent                          bool
	}{observation.File, observation.Scene, observation.Mood, observation.VisibleText, observation.Speech, originOrNil(observation.Objects), originOrNil(observation.Events), observation.PeoplePresent}
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return OriginResultIdentity{ContentHash: hex.EncodeToString(sum[:])}
}

// StorylineFingerprint fences only the plan being replaced. Observation turns
// and unrelated brief changes cannot masquerade as manual plan changes.
func StorylineFingerprint(storyline *Storyline) string {
	var value any
	if storyline != nil {
		paragraphs := slices.Clone(storyline.Paragraphs)
		for i := range paragraphs {
			paragraphs[i].Files = originOrNil(paragraphs[i].Files)
		}
		value = struct {
			Paragraphs   []StorylineParagraph
			EditedByHand bool
			MadeWith     []string
			Origins      *PlanOriginReview
		}{paragraphs, storyline.EditedByHand, originOrNil(storyline.MadeWith), storyline.Origins}
	}
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func originOrNil[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}

// ValidateResultOriginReview adds the semantic source-kind/attachment fence to
// structural quote validation. It certifies neither factual nor visual accuracy.
func ValidateResultOriginReview(content PostContent, current OriginResultIdentity, review *OriginReview) OriginResolution {
	resolved := ValidateOriginReview(content, current, review)
	catalog := make(map[string]OriginSource, len(resolved.Review.Sources))
	for _, source := range resolved.Review.Sources {
		catalog[source.ID] = source
	}
	kept := resolved.Review.Spans[:0]
	for _, span := range resolved.Review.Spans {
		if alignmentSourceCompatible(content, span, catalog) {
			kept = append(kept, span)
		}
	}
	resolved.Review.Spans = kept
	return resolved
}

type GeneratedOriginResult struct {
	Content                 PostContent
	Language                Language
	Annotations             *WriteAnnotations
	Origins                 *OriginReview
	ExpectedContentRevision int64
	ExpectedPlanFingerprint *string
}

type StorylineOriginResult struct {
	Storyline               Storyline
	ExpectedContentRevision int64
	ExpectedInputRevision   *int64
	ExpectedPlanFingerprint *string
}

type OriginResultStore interface {
	PublishGeneratedResult(context.Context, string, string, GeneratedOriginResult, time.Time) (OriginResultIdentity, error)
	PublishStorylineResult(context.Context, string, string, StorylineOriginResult, time.Time) error
	ReadOriginReview(context.Context, string, string, OriginResultIdentity) (*OriginReview, error)
}

type OriginResultService struct {
	store OriginResultStore
	now   func() time.Time
}

func NewOriginResultService(store OriginResultStore) *OriginResultService {
	if store == nil {
		panic("post: origin result store is required")
	}
	return &OriginResultService{store: store, now: time.Now}
}

func (s *OriginResultService) PublishGeneratedResult(ctx context.Context, userID, slug string, result GeneratedOriginResult) (OriginResultIdentity, error) {
	return s.store.PublishGeneratedResult(ctx, userID, slug, result, s.now())
}

func (s *OriginResultService) PublishStorylineResult(ctx context.Context, userID, slug string, result StorylineOriginResult) error {
	return s.store.PublishStorylineResult(ctx, userID, slug, result, s.now())
}

func (s *OriginResultService) ReadOriginReview(ctx context.Context, userID, slug string, current OriginResultIdentity) (*OriginReview, error) {
	return s.store.ReadOriginReview(ctx, userID, slug, current)
}
