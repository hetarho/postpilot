package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"time"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store/sqlc"
)

func ownedOriginPost(ctx context.Context, q *sqlc.Queries, userID, slug string) (post.Post, error) {
	row, err := q.GetPost(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return post.Post{}, post.ErrNotFound
	}
	if err != nil {
		return post.Post{}, err
	}
	value, err := toPost(row)
	if err != nil {
		return post.Post{}, err
	}
	if value.UserID != userID {
		return post.Post{}, post.ErrForbidden
	}
	return value, nil
}

func originAttachments(ctx context.Context, q *sqlc.Queries, slug string) ([]post.Image, []post.Video, []string, error) {
	images, err := q.ListImagesByPost(ctx, slug)
	if err != nil {
		return nil, nil, nil, err
	}
	photos := make([]post.Image, len(images))
	names := make([]string, 0, len(images))
	for i, row := range images {
		photos[i], err = toImage(row)
		if err != nil {
			return nil, nil, nil, err
		}
		names = append(names, photos[i].Filename)
	}
	rows, err := q.ListVideosByPost(ctx, slug)
	if err != nil {
		return nil, nil, nil, err
	}
	videos := make([]post.Video, len(rows))
	for i, row := range rows {
		videos[i], err = toVideo(row)
		if err != nil {
			return nil, nil, nil, err
		}
		names = append(names, videos[i].Filename)
	}
	return photos, videos, names, nil
}

func rawOriginIdentity(value any, revision int64) post.OriginResultIdentity {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return post.OriginResultIdentity{ContentRevision: revision, ContentHash: hex.EncodeToString(sum[:])}
}

func originSourcesAvailable(sources []post.OriginSource, attached []string) []post.OriginSource {
	copy := slices.Clone(sources)
	for i := range copy {
		if copy[i].Kind == post.OriginSourceVisualObservation && !slices.Contains(attached, copy[i].AttachmentFilename) {
			copy[i].Available = false
		}
	}
	return copy
}

func originSourcesWithIncarnations(sources []post.OriginSource, photos []post.Image, videos []post.Video) []post.OriginSource {
	current := make(map[string]string, len(photos)+len(videos))
	for _, image := range photos {
		current[image.Filename] = image.ID
	}
	for _, video := range videos {
		current[video.Filename] = video.ID
	}
	copy := slices.Clone(sources)
	for i := range copy {
		if copy[i].Kind == post.OriginSourceVisualObservation && (copy[i].AttachmentID == "" || current[copy[i].AttachmentFilename] != copy[i].AttachmentID) {
			copy[i].Available = false
		}
	}
	return copy
}

func observationMatchesIncarnation(observation post.Observation, currentID, removedID string) bool {
	if currentID == "" || currentID == removedID || observation.Origins == nil || observation.Origins.Version != post.OriginVersion || observation.Origins.Result != post.ObservationOriginIdentity(observation) {
		return false
	}
	matched := false
	for _, source := range observation.Origins.Sources {
		if source.Kind != post.OriginSourceVisualObservation {
			continue
		}
		if !source.Available || source.AttachmentFilename != observation.File || source.AttachmentID != currentID {
			return false
		}
		matched = true
	}
	return matched
}

func validatedStoredOrigins(content post.PostContent, current post.OriginResultIdentity, review *post.OriginReview, attached []string) *post.OriginReview {
	if review == nil || review.Version != post.OriginVersion || review.Result != current {
		return nil
	}
	copy := cloneOriginReview(review)
	copy.Sources = originSourcesAvailable(copy.Sources, attached)
	resolved := post.ValidateResultOriginReview(content, current, copy).Review
	return &resolved
}

// Accept only an exact input staging identity or the exact durable canonical
// identity. Publication assigns the next authoritative revision/hash; metadata
// invalidity alone produces an unconfirmed result, never a canonical refusal.
func publishableOrigins(content post.PostContent, review *post.OriginReview, next int64, attached []string) *post.OriginReview {
	if review == nil {
		return nil
	}
	raw := rawOriginIdentity(content, 0)
	normalized := post.ContentOriginIdentity(content, 0)
	if review.Result != raw && review.Result != normalized {
		return nil
	}
	resolved := validatedStoredOrigins(content, review.Result, review, attached)
	if resolved != nil {
		resolved.Result = post.ContentOriginIdentity(content, next)
	}
	return resolved
}

func publishablePlanOrigins(paragraphs []post.StorylineParagraph, review *post.PlanOriginReview, attached []string) *post.PlanOriginReview {
	if review == nil || review.Version != post.OriginVersion {
		return nil
	}
	if review.Result != rawOriginIdentity(paragraphs, 0) && review.Result != post.PlanOriginIdentity(paragraphs) {
		return nil
	}
	content := post.PostContent{}
	for _, paragraph := range paragraphs {
		content.Blocks = append(content.Blocks, post.Block{Type: post.BlockText, Content: paragraph.Text})
	}
	identity := post.ContentOriginIdentity(content, 0)
	projected := &post.OriginReview{Version: post.OriginVersion, Result: identity, Sources: review.Sources}
	for _, span := range review.Spans {
		index := span.ParagraphIndex
		projected.Spans = append(projected.Spans, post.OriginSpan{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockContent, BlockIndex: &index}, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: span.SourceRefs, ReviewState: span.ReviewState})
	}
	resolved := validatedStoredOrigins(content, identity, projected, attached)
	if resolved == nil {
		return nil
	}
	out := &post.PlanOriginReview{Version: post.OriginVersion, Result: post.PlanOriginIdentity(paragraphs), Sources: resolved.Sources}
	for _, span := range resolved.Spans {
		out.Spans = append(out.Spans, post.PlanOriginSpan{ParagraphIndex: *span.Field.BlockIndex, Start: span.Start, End: span.End, Quote: span.Quote, Category: span.Category, SourceRefs: span.SourceRefs, ReviewState: span.ReviewState})
	}
	return out
}

func (s *Store) PublishGeneratedResult(ctx context.Context, userID, slug string, in post.GeneratedOriginResult, now time.Time) (post.OriginResultIdentity, error) {
	if !in.Language.Valid() {
		return post.OriginResultIdentity{}, post.ErrLanguageRequired
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	found, err := ownedOriginPost(ctx, q, userID, slug)
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	if found.Status == post.StatusPublished {
		return post.OriginResultIdentity{}, post.ErrPostPublished
	}
	photos, videos, attached, err := originAttachments(ctx, q, slug)
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	if err := post.ValidateContent(in.Content, photos, videos); err != nil {
		return post.OriginResultIdentity{}, err
	}
	annotations := post.WriteAnnotations{Nouns: found.ContentNouns, Storyline: found.Storyline}
	if in.Annotations != nil {
		annotations.Nouns = in.Annotations.Nouns
		if in.Annotations.Storyline != nil {
			annotations.Storyline = in.Annotations.Storyline
			if len(annotations.Storyline.Paragraphs) == 0 {
				annotations.Storyline = nil
			}
		}
	}
	// A prose revision can retain an owner-edited current plan. Only a supplied
	// machine replacement must obey the machine-plan authorship contract.
	if in.Annotations != nil && in.Annotations.Storyline != nil {
		if err := post.ValidateTestStoryline(annotations.Storyline, photos, videos); err != nil {
			return post.OriginResultIdentity{}, err
		}
	}
	content, err := marshalContent(in.Content)
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	canonical, err := unmarshalContent(content)
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	incomingOrigins := cloneOriginReview(in.Origins)
	if incomingOrigins != nil {
		incomingOrigins.Sources = originSourcesWithIncarnations(incomingOrigins.Sources, photos, videos)
	}
	origins := publishableOrigins(in.Content, incomingOrigins, found.ContentRevision+1, attached)
	if origins != nil {
		origins.Result = post.ContentOriginIdentity(*canonical, found.ContentRevision+1)
	}
	storyline, err := marshalStoryline(annotations.Storyline)
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	var planOrigins *post.PlanOriginReview
	if annotations.Storyline != nil {
		incomingPlan := clonePlanOrigins(annotations.Storyline.Origins)
		if incomingPlan != nil {
			incomingPlan.Sources = originSourcesWithIncarnations(incomingPlan.Sources, photos, videos)
		}
		planOrigins = publishablePlanOrigins(annotations.Storyline.Paragraphs, incomingPlan, attached)
	}
	nouns, err := marshalNouns(annotations.Nouns)
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	// A lost response can replay only the exact committed machine result. A later
	// manual edit moves the revision away from its baseline, so it cannot be undone.
	if found.Content != nil && found.Status == post.StatusReview && found.MachineBaselineRevision == found.ContentRevision && found.ContentLanguage != nil && *found.ContentLanguage == in.Language {
		stored, _ := marshalContent(*found.Content)
		if origins != nil {
			origins.Result = post.ContentOriginIdentity(*canonical, found.ContentRevision)
		}
		currentStory, _ := marshalStoryline(found.Storyline)
		var priorPlan *post.PlanOriginReview
		if found.Storyline != nil {
			priorPlan = found.Storyline.Origins
		}
		if stored == content && slices.Equal(found.ContentNouns, annotations.Nouns) && currentStory == storyline && reflect.DeepEqual(found.ContentOrigins, origins) && reflect.DeepEqual(priorPlan, planOrigins) {
			return post.ContentOriginIdentity(*found.Content, found.ContentRevision), nil
		}
		if origins != nil {
			origins.Result = post.ContentOriginIdentity(*canonical, found.ContentRevision+1)
		}
	}
	if found.ContentRevision != in.ExpectedContentRevision {
		return post.OriginResultIdentity{}, post.ErrStaleContentRevision
	}
	if in.ExpectedPlanFingerprint != nil && post.StorylineFingerprint(found.Storyline) != *in.ExpectedPlanFingerprint {
		return post.OriginResultIdentity{}, post.ErrStaleContentRevision
	}
	n, err := q.PublishPostOriginContent(ctx, sqlc.PublishPostOriginContentParams{Storyline: storyline, Content: sql.NullString{String: content, Valid: true}, ContentLanguage: sql.NullString{String: string(in.Language), Valid: true}, ContentNouns: nouns, StorylineOrigins: marshalPlanOrigins(planOrigins), ContentOrigins: marshalOriginReview(origins), UpdatedAt: formatTime(now), Slug: slug, UserID: userID, ExpectedContentRevision: in.ExpectedContentRevision})
	if err != nil {
		return post.OriginResultIdentity{}, err
	}
	if n != 1 {
		return post.OriginResultIdentity{}, post.ErrStaleContentRevision
	}
	if err := tx.Commit(); err != nil {
		return post.OriginResultIdentity{}, err
	}
	return post.ContentOriginIdentity(*canonical, found.ContentRevision+1), nil
}

func (s *Store) PublishStorylineResult(ctx context.Context, userID, slug string, in post.StorylineOriginResult, now time.Time) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.write.WithTx(tx)
	found, err := ownedOriginPost(ctx, q, userID, slug)
	if err != nil {
		return err
	}
	if found.Status == post.StatusPublished {
		return post.ErrPostPublished
	}
	if found.ContentRevision != in.ExpectedContentRevision {
		return post.ErrStaleContentRevision
	}
	photos, videos, attached, err := originAttachments(ctx, q, slug)
	if err != nil {
		return err
	}
	var plan *post.Storyline
	if len(in.Storyline.Paragraphs) > 0 {
		plan = &in.Storyline
	}
	if err := post.ValidateTestStoryline(plan, photos, videos); err != nil {
		return err
	}
	encoded, err := marshalStoryline(plan)
	if err != nil {
		return err
	}
	var origins *post.PlanOriginReview
	if plan != nil {
		incoming := clonePlanOrigins(plan.Origins)
		if incoming != nil {
			incoming.Sources = originSourcesWithIncarnations(incoming.Sources, photos, videos)
		}
		origins = publishablePlanOrigins(plan.Paragraphs, incoming, attached)
	}
	currentPlan, _ := marshalStoryline(found.Storyline)
	var storedOrigins *post.PlanOriginReview
	if found.Storyline != nil {
		storedOrigins = found.Storyline.Origins
	}
	if currentPlan == encoded && reflect.DeepEqual(storedOrigins, origins) {
		return nil
	}
	if in.ExpectedPlanFingerprint != nil {
		if post.StorylineFingerprint(found.Storyline) != *in.ExpectedPlanFingerprint {
			return post.ErrStaleContentRevision
		}
	} else if in.ExpectedInputRevision == nil || found.InputRevision != *in.ExpectedInputRevision {
		return post.ErrStaleContentRevision
	}
	// The validated plan fingerprint permits only this exact current transaction;
	// SQL still carries an input revision guard against any intervening write.
	expectedInput := found.InputRevision
	changed, err := q.PublishPostOriginStoryline(ctx, sqlc.PublishPostOriginStorylineParams{Storyline: encoded, StorylineOrigins: marshalPlanOrigins(origins), UpdatedAt: formatTime(now), Slug: slug, UserID: userID, ExpectedContentRevision: in.ExpectedContentRevision, ExpectedInputRevision: expectedInput})
	if err != nil {
		return err
	}
	if changed != 1 {
		return post.ErrStaleContentRevision
	}
	return tx.Commit()
}

func (s *Store) ReadOriginReview(ctx context.Context, userID, slug string, identity post.OriginResultIdentity) (*post.OriginReview, error) {
	found, err := ownedOriginPost(ctx, s.read, userID, slug)
	if err != nil {
		return nil, err
	}
	if found.Content == nil || post.ContentOriginIdentity(*found.Content, found.ContentRevision) != identity {
		return nil, nil
	}
	photos, videos, attached, err := originAttachments(ctx, s.read, slug)
	if err != nil {
		return nil, err
	}
	prior := cloneOriginReview(found.ContentOrigins)
	if prior != nil {
		prior.Sources = originSourcesWithIncarnations(prior.Sources, photos, videos)
	}
	return validatedStoredOrigins(*found.Content, identity, prior, attached), nil
}

func withdrawOriginIncarnation(ctx context.Context, q *sqlc.Queries, slug, mediaID string) error {
	// Captured prompts may include interpretation of several attachments in one
	// fragment. Erase the complete private selection rather than retaining or
	// retargeting any withdrawn source's request text.
	if err := q.PurgeWithdrawnPostRequestCaptures(ctx, slug); err != nil {
		return err
	}
	row, err := q.GetPost(ctx, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	content := unmarshalOriginReview(row.ContentOrigins)
	plan := unmarshalPlanOrigins(row.StorylineOrigins)
	if content != nil {
		for i := range content.Sources {
			if content.Sources[i].Kind == post.OriginSourceVisualObservation && content.Sources[i].AttachmentID == mediaID {
				content.Sources[i].Available = false
			}
		}
	}
	if plan != nil {
		for i := range plan.Sources {
			if plan.Sources[i].Kind == post.OriginSourceVisualObservation && plan.Sources[i].AttachmentID == mediaID {
				plan.Sources[i].Available = false
			}
		}
	}
	_, err = q.SetPostOriginAvailability(ctx, sqlc.SetPostOriginAvailabilityParams{ContentOrigins: marshalOriginReview(content), StorylineOrigins: marshalPlanOrigins(plan), Slug: slug, UserID: row.UserID})
	return err
}
