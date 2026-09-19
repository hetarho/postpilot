package main

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/publishing"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

type billingMailer struct{ mailer auth.Mailer }

func (m billingMailer) Send(ctx context.Context, to, subject, text string) error {
	return m.mailer.Send(ctx, auth.Mail{To: to, Subject: subject, Text: text})
}

type publishingPosts struct{ service *post.Service }

func (a publishingPosts) PostIdentity(ctx context.Context, userID, postSlug string) (time.Time, error) {
	createdAt, err := a.service.PostIdentity(ctx, userID, postSlug)
	if err != nil {
		switch {
		case errors.Is(err, post.ErrNotFound):
			return time.Time{}, publishing.ErrNotFound
		case errors.Is(err, post.ErrForbidden):
			return time.Time{}, publishing.ErrForbidden
		default:
			return time.Time{}, err
		}
	}
	return createdAt, nil
}

func (a publishingPosts) PublishingSnapshot(ctx context.Context, userID, postSlug string) (publishing.PostSnapshot, error) {
	snapshot, err := a.service.PublishingSnapshot(ctx, userID, postSlug)
	if err != nil {
		switch {
		case errors.Is(err, post.ErrNotFound):
			return publishing.PostSnapshot{}, publishing.ErrNotFound
		case errors.Is(err, post.ErrForbidden):
			return publishing.PostSnapshot{}, publishing.ErrForbidden
		case errors.Is(err, post.ErrPostNotFinalized):
			return publishing.PostSnapshot{}, publishing.ErrPostNotFinalized
		case errors.Is(err, post.ErrInvalidContent), errors.Is(err, post.ErrLanguageRequired):
			return publishing.PostSnapshot{}, publishing.ErrInvalid
		default:
			return publishing.PostSnapshot{}, err
		}
	}
	content := publishing.Content{Title: snapshot.Content.Title, Summary: snapshot.Content.Summary, Tags: append([]string(nil), snapshot.Content.Tags...)}
	content.Blocks = make([]publishing.Block, 0, len(snapshot.Content.Blocks))
	for _, block := range snapshot.Content.Blocks {
		content.Blocks = append(content.Blocks, publishing.Block{Type: publishing.BlockType(block.Type), Content: block.Content,
			Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: append([]string(nil), block.Items...)})
	}
	images := make([]publishing.SnapshotImage, 0, len(snapshot.Images))
	for _, image := range snapshot.Images {
		images = append(images, publishing.SnapshotImage{Filename: image.Filename, Key: image.Key, Bytes: image.Bytes})
	}
	return publishing.PostSnapshot{PostSlug: snapshot.PostSlug, UserID: snapshot.UserID, CreatedAt: snapshot.CreatedAt, Content: content,
		ContentRevision: snapshot.ContentRevision, FinalizedRevision: snapshot.FinalizedRevision, Images: images,
		TargetLanguage: publishing.Language(snapshot.TargetLanguage), ContentLanguage: publishing.Language(snapshot.ContentLanguage),
		VoiceSourceLanguage: publishing.Language(snapshot.VoiceSourceLanguage)}, nil
}

var _ publishing.PostSnapshots = publishingPosts{}

// postVoices, postCandidateLinks and postPublications are read by the post context,
// which is constructed before voice, guideline and publishing (publishing reads post).
// Each names the service directly when a test hands one over, and otherwise resolves it
// through the contexts at call time, after buildContexts has finished.
type postVoices struct {
	service *voice.Service
	app     *contexts
}

func (a postVoices) voices() *voice.Service {
	if a.service != nil {
		return a.service
	}
	return a.app.voice
}

func (a postVoices) Voices(ctx context.Context, userID string) ([]post.VoiceRef, error) {
	voices, err := a.voices().ListVoices(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]post.VoiceRef, 0, len(voices))
	for _, v := range voices {
		out = append(out, post.VoiceRef{ID: v.ID, Name: v.Name, Deleted: v.Deleted(), SourceLanguage: post.Language(v.SourceLanguage)})
	}
	return out, nil
}

// postTemplates adapts the template directory for the post context. Unlike voices there are no
// tombstones: a deleted template simply stops being listed, and the composite foreign key has
// already cleared the assignments that named it.
type postTemplates struct{ service *template.Service }

func (a postTemplates) Templates(ctx context.Context, userID string) ([]post.TemplateRef, error) {
	templates, err := a.service.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]post.TemplateRef, 0, len(templates))
	for _, p := range templates {
		// The two generation numbers travel with the ref because an assignment seeds the post's
		// own options from them (TEMPLATE-48). Nothing else reads them: no prompt, payload or
		// read model ever sees a template's number.
		out = append(out, post.TemplateRef{
			ID: p.ID, Name: p.Name, TargetLength: p.TargetLength, TagCount: p.TagCount,
		})
	}
	return out, nil
}

// generationTemplates hands the generation context the frozen text of one owned template. It is
// consulted once per enqueue; no handler ever calls it.
type guidelineTemplates struct{ service *template.Service }

func (a guidelineTemplates) Templates(ctx context.Context, userID string) ([]guideline.TemplateRef, error) {
	templates, err := a.service.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]guideline.TemplateRef, 0, len(templates))
	for _, p := range templates {
		out = append(out, guideline.TemplateRef{ID: p.ID, Name: p.Name})
	}
	return out, nil
}

// generationGuidelines hands the generation context the applicable ordered texts for one
// post. It is consumed only at enqueue, to freeze them into the durable payload.
type postCandidateLinks struct {
	service *guideline.Service
	app     *contexts
}

func (a postCandidateLinks) guidelines() *guideline.Service {
	if a.service != nil {
		return a.service
	}
	return a.app.guideline
}

func (a postCandidateLinks) DetachPost(ctx context.Context, userID, postSlug string) error {
	return a.guidelines().DetachCandidatePost(ctx, userID, postSlug)
}

// RenderedFor is the whole seam between the two contexts: generation hands over the account,
// the template id and the frozen attachment order, and receives prompt text. It never learns
// the grammar, and the template context never learns what a job is.
//
// The expansion bound is enforced on the other side, so an error here is a real refusal that
// must stop the start rather than fall back to "no template".
type postJobFinder struct {
	queue *job.Queue
}

// postPublications adapts the publishing context's in-flight query for the post context,
// which speaks only in primitives across this boundary.
type postPublications struct {
	service *publishing.Service
	app     *contexts
}

func (a postPublications) publishing() *publishing.Service {
	if a.service != nil {
		return a.service
	}
	return a.app.publishing
}

func (a postPublications) LiveForPost(ctx context.Context, userID, slug string, createdAt time.Time) (bool, error) {
	return a.publishing().HasLiveJobForPost(ctx, userID, slug, createdAt)
}

func (a postJobFinder) ActiveForPost(ctx context.Context, slug string) (*post.ActiveJob, error) {
	found, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: post.JobSubject, ID: slug}, job.Filter{})
	if err != nil || found == nil {
		return nil, err
	}
	postSlug := found.Subject(post.JobSubject)
	return &post.ActiveJob{
		ID: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage,
		ProgressDone: found.ProgressDone, ProgressTotal: found.ProgressTotal,
		Failure:  postFailure(found.Failure),
		PostSlug: postSlug, ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		TargetLanguage: post.Language(found.TargetLanguage),
		CreatedAt:      found.CreatedAt, UpdatedAt: found.UpdatedAt,
	}, nil
}

func generationFailure(found *job.Failure) *generation.Failure {
	if found == nil || found.Reason == "" {
		return nil
	}
	return &generation.Failure{
		Reason: found.Reason, Params: cloneStringMap(found.Params), TechnicalDetail: found.TechnicalDetail,
	}
}

func postFailure(found *job.Failure) *post.Failure {
	if found == nil || found.Reason == "" {
		return nil
	}
	return &post.Failure{
		Reason: found.Reason, Params: cloneStringMap(found.Params), TechnicalDetail: found.TechnicalDetail,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
