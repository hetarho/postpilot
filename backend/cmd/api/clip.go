package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	clipai "github.com/postpilot/backend/internal/clip/ai"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	clipmedia "github.com/postpilot/backend/internal/clip/media"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/storage"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// clipTxPorts is the one place that knows which store constructors wrap the
// writer transaction the clip sagas commit in (ARCH-6): the job and clip stores
// on the same *sql.Tx, and the credit admission over the ledger's tx-scoped
// store so a hold lands with the job row it guards.
func clipTxPorts(ledger *usage.Service, registry *llm.Registry, plans *auth.Service) clipapp.Binder {
	return func(tx *sql.Tx) clipapp.Ports {
		ports := clipapp.Ports{Jobs: jobstore.NewTx(tx), Clips: clipstore.NewTx(tx)}
		if ledger != nil {
			ports.Admission = jobAdmission{ledger: ledger.WithStore(usagestore.NewTx(tx)), registry: registry, plans: plans}
		}
		return ports
	}
}

func clipBudgets(cfg clipai.Config) clipapp.Budgets {
	return clipapp.Budgets{ObserveCompletionTokens: cfg.ObserveCompletionTokens, FlowCompletionTokens: cfg.FlowCompletionTokens, NarrationCompletionTokens: cfg.NarrationCompletionTokens, ObserveReasoning: cfg.ObserveReasoning, PlanReasoning: cfg.PlanReasoning}
}

func newClipGeneration(ctx context.Context, cfg *config.Config, store *clipstore.Store, projects *clip.Service, sources *clip.SourceService, bucket *storage.Bucket, media *clipmedia.Adapter, models meteredRegistry, queue *job.Queue, writer *sql.DB, bind clipapp.Binder) (*clip.GenerationService, error) {
	renderer, err := clipmedia.NewRenderer(media, config.ClipRender(cfg))
	if err != nil {
		return nil, err
	}
	aiConfig := config.ClipAI(cfg)
	planner, err := clipai.New(clipModels{models}, renderer, aiConfig)
	if err != nil {
		return nil, err
	}
	service := clip.NewGenerationService(store, projects, sources, bucket, media, planner, renderer, clipapp.NewJobs(queue), config.ClipGeneration(cfg))
	finisher := clipapp.NewFinisher(writer, bind, jobstore.New(writer, writer), store, nil)
	service.WithFinisher(finisher)
	service.WithCredits(clipapp.NewPricing(models.Registry, clipBudgets(aiConfig)), clipapp.NewAccounting(models.ledger))
	service.WithAdmission(clipapp.NewModelAdmission(models.Registry, clipBudgets(aiConfig)))
	projects.SetGeneration(service)
	projects.SetFinalizer(clipapp.NewFinalizer(writer, bind, store, config.ClipRender(cfg), nil))
	if _, err = queue.SweepUnactivatedClips(ctx); err != nil {
		return nil, err
	}
	if err := finisher.Recover(ctx); err != nil {
		return nil, err
	}
	queue.Register(job.KindGenerateClip, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.Run(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, progress)
	}))
	queue.Register(job.KindRenderClip, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunRender(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, progress)
	}))
	queue.Register(job.KindReviseClip, metered(func(ctx context.Context, j job.Job, progress job.Progress) error {
		return service.RunRevision(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, progress)
	}))
	for _, kind := range []string{job.KindGenerateClip, job.KindRenderClip, job.KindReviseClip} {
		queue.OnTerminal(kind, func(ctx context.Context, j job.Job, at time.Time) error {
			return sources.ReleaseAttempt(ctx, j.UserID, j.ID, at)
		})
	}
	return service, nil
}

type clipModels struct{ meteredRegistry }

func (m clipModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return m.Lookup(ref) }
