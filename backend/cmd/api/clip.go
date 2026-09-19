package main

import (
	"context"
	"database/sql"

	"github.com/postpilot/backend/internal/auth"
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

func newClipGeneration(ctx context.Context, cfg *config.Config, store *clipstore.Store, projects *clipapp.Service, sources *clipapp.SourceService, bucket *storage.Bucket, media *clipmedia.Adapter, models meteredRegistry, queue *job.Queue, writer *sql.DB, bind clipapp.Binder) (*clipapp.GenerationService, error) {
	renderer, err := clipmedia.NewRenderer(media, config.ClipRender(cfg))
	if err != nil {
		return nil, err
	}
	aiConfig := config.ClipAI(cfg)
	planner, err := clipai.New(clipModels{models}, renderer, aiConfig)
	if err != nil {
		return nil, err
	}
	finisher := clipapp.NewFinisher(writer, bind, jobstore.New(writer, writer), store, nil)
	service := clipapp.NewGenerationService(store, projects, sources, bucket, media, planner, renderer, clipapp.NewJobs(queue), config.ClipGeneration(cfg), clipapp.GenerationDeps{
		Finisher:   finisher,
		Pricing:    clipapp.NewPricing(models.Registry, clipBudgets(aiConfig)),
		Accounting: clipapp.NewAccounting(models.ledger),
		Admission:  clipapp.NewModelAdmission(models.Registry, clipBudgets(aiConfig)),
	})
	if _, err = queue.SweepUnactivatedClips(ctx); err != nil {
		return nil, err
	}
	if err := finisher.Recover(ctx); err != nil {
		return nil, err
	}
	return service, nil
}

type clipModels struct{ meteredRegistry }

func (m clipModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return m.Lookup(ref) }
