package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	clipai "github.com/postpilot/backend/internal/clip/ai"
	clipmedia "github.com/postpilot/backend/internal/clip/media"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/storage"
)

func newClipGeneration(ctx context.Context, cfg *config.Config, store *clipstore.Store, projects *clip.Service, sources *clip.SourceService, bucket *storage.Bucket, media *clipmedia.Adapter, models meteredRegistry, queue *job.Queue) (*clip.GenerationService, error) {
	renderer, err := clipmedia.NewRenderer(media, config.ClipRender(cfg))
	if err != nil {
		return nil, err
	}
	planner, err := clipai.New(clipModels{models}, renderer, config.ClipAI(cfg))
	if err != nil {
		return nil, err
	}
	service := clip.NewGenerationService(store, projects, sources, bucket, media, planner, renderer, clipJobs{queue}, config.ClipGeneration(cfg))
	service.WithCredits(clipQuotePricing{registry: models.Registry, cfg: config.ClipAI(cfg)}, clipAccounting{ledger: models.ledger})
	projects.SetGeneration(service)
	if _, err = queue.SweepUnactivatedClips(ctx); err != nil {
		return nil, err
	}
	queue.Register(job.KindGenerateClip, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.Run(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, progress)
	}))
	queue.Register(job.KindRenderClip, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunRender(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, progress)
	}))
	return service, nil
}

type clipJobs struct{ queue *job.Queue }
type clipModels struct{ meteredRegistry }

func (m clipModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return m.Lookup(ref) }

func (a clipJobs) Enqueue(ctx context.Context, s clip.GenerationStart) (string, error) {
	kind := job.KindGenerateClip
	if s.RenderOnly {
		kind = job.KindRenderClip
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{Kind: kind, NonMetered: s.RenderOnly, UserID: s.UserID, ClipProjectID: s.ProjectID, ObserveModel: s.Observe, WriteModel: s.Write, Payload: s.Payload})
	if errors.Is(err, job.ErrActiveConflict) {
		return "", clip.ErrBusy
	}
	if errors.Is(err, job.ErrInvalidTarget) {
		return "", clip.ErrNotFound
	}
	return id, err
}
func (a clipJobs) Activate(ctx context.Context, user, id string) error {
	return a.queue.ActivateClip(ctx, user, id)
}
func (a clipJobs) FailQueued(ctx context.Context, user, id string) (bool, error) {
	return a.queue.FailQueued(ctx, id, user, job.Failure{Reason: "CLIP_PROCESSING_FAILED"})
}
func (a clipJobs) Active(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.ActiveForClip(ctx, user, id)
	return clipJob(j), err
}
func (a clipJobs) Get(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	j, err := a.queue.Get(ctx, id, user)
	if errors.Is(err, job.ErrNotFound) {
		return nil, nil
	}
	return clipJob(j), err
}
func clipJob(j *job.JobSummary) *clip.ClipJob {
	if j == nil {
		return nil
	}
	return &clip.ClipJob{ID: j.ID, Status: j.Status, Stage: j.Stage}
}
