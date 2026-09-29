package main

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/storage"
	"github.com/postpilot/backend/internal/voice"
)

type voiceModels struct {
	registry meteredRegistry
}

func (a voiceModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return a.registry.Lookup(ref) }

func (a voiceModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return a.registry.Complete(ctx, ref, request)
}

type voiceJobs struct {
	queue  *job.Queue
	budget config.LLMCompletionBudget
}

func (a voiceJobs) Enqueue(ctx context.Context, request voice.AnalysisJobRequest) (string, error) {
	subjects, guards := postVoiceWork(job.KindAnalyzeVoice, request.UserID, "", request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindAnalyzeVoice, UserID: request.UserID, Subjects: subjects, Guards: guards, WriteModel: request.WriteModel,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", voice.ErrVoiceDeleted
	}
	return id, err
}

// EnqueueCheck starts one 검증 (VOICE-43) through the shared admission (QUOTA-13), priced as the
// one write call it makes at the write stage's floor; the payload names the check.
func (a voiceJobs) EnqueueCheck(ctx context.Context, request voice.CheckJobRequest) (string, error) {
	subjects, guards := postVoiceWork(job.KindCheckVoice, request.UserID, "", request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindCheckVoice, UserID: request.UserID, Subjects: subjects, Guards: guards, WriteModel: request.WriteModel,
		Payload:      []byte(request.CheckID),
		PricingCalls: []job.PlannedCall{{Ref: request.WriteModel, Count: 1, CompletionTokens: a.budget.WriteFloor}},
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", voice.ErrVoiceDeleted
	}
	return id, err
}

func (a voiceJobs) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{Kind: kind})
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
}

func (a voiceJobs) HasActiveForVoice(ctx context.Context, voiceID string) (bool, error) {
	return a.queue.HasActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{})
}

// voiceObjects is the bucket as the voice context's own ObjectStore port (ARCH-6): the same
// private bucket and prefix rules as a post photo, with the storage types translated here.
type voiceObjects struct{ bucket *storage.Bucket }

func (a voiceObjects) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	return a.bucket.PresignPut(ctx, key, contentType, ttl)
}

func (a voiceObjects) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return a.bucket.PresignGet(ctx, key, ttl)
}

func (a voiceObjects) Head(ctx context.Context, key string) (voice.ObjectHead, error) {
	head, err := a.bucket.Head(ctx, key)
	if errors.Is(err, post.ErrObjectNotFound) {
		return voice.ObjectHead{}, voice.ErrObjectNotFound
	}
	if err != nil {
		return voice.ObjectHead{}, err
	}
	return voice.ObjectHead{Size: head.Size, ContentType: head.ContentType}, nil
}

// Read is a photo's bytes for a 검증's call, capped by the bucket at the image limit.
func (a voiceObjects) Read(ctx context.Context, key string) ([]byte, error) {
	return a.bucket.ReadObject(ctx, key)
}

func (a voiceObjects) Delete(ctx context.Context, key string) error { return a.bucket.Delete(ctx, key) }

func (a voiceObjects) List(ctx context.Context, prefix string) ([]voice.StoredObject, error) {
	objects, err := a.bucket.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := make([]voice.StoredObject, 0, len(objects))
	for _, object := range objects {
		out = append(out, voice.StoredObject{Key: object.Key, LastModified: object.LastModified})
	}
	return out, nil
}

// voicePosts hands the voice context one owned post's blocks to count (POST-102). The post's
// block types are translated here, so the voice context never reads post tables or types.
type voicePosts struct{ service *post.Service }

func (a voicePosts) PostForFingerprint(ctx context.Context, userID, slug string) (string, int64, []voice.Block, error) {
	found, err := a.service.CurrentContent(ctx, userID, slug)
	switch {
	case errors.Is(err, post.ErrNotFound):
		return "", 0, nil, voice.ErrPostNotFound
	case errors.Is(err, post.ErrForbidden):
		return "", 0, nil, voice.ErrPostForbidden
	case err != nil:
		return "", 0, nil, err
	}
	if found.Content == nil {
		return found.VoiceID, found.ContentRevision, nil, nil
	}
	return found.VoiceID, found.ContentRevision, fingerprintBlocks(found.Content.Blocks), nil
}

// fingerprintBlocks keeps the prose blocks the fingerprint reads — TEXT, HEADING, LIST (its
// items) and QUOTE — and skips IMAGE and VIDEO, captions included (VOICE-62).
func fingerprintBlocks(blocks []post.Block) []voice.Block {
	out := make([]voice.Block, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case post.BlockText:
			out = append(out, voice.Block{Type: voice.BlockText, Content: block.Content})
		case post.BlockHeading:
			out = append(out, voice.Block{Type: voice.BlockHeading, Content: block.Content})
		case post.BlockQuote:
			out = append(out, voice.Block{Type: voice.BlockQuote, Content: block.Content})
		case post.BlockList:
			out = append(out, voice.Block{Type: voice.BlockList, Items: append([]string(nil), block.Items...)})
		}
	}
	return out
}
