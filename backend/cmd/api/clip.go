package main

import (
	"context"
	"database/sql"

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
		jobs := jobstore.NewTx(tx, jobKinds())
		clips := clipstore.NewTx(tx)
		ports := clipapp.Ports{Jobs: jobs, Waits: jobs, Clips: clips, Media: clips, Stages: clips, Publication: clips}
		if ledger != nil {
			ports.Admission = clipAdmission{jobAdmission{ledger: ledger.WithStore(usagestore.NewTx(tx)), registry: registry, plans: plans}}
		}
		return ports
	}
}

// clipEnvironment is where the parsed env meets the clip context's own limits
// (ARCH-21): platform/config owns the values a deployment may move, the clip
// package owns the product rules, and this is the one merge point.
func clipEnvironment(cfg *config.Config) clip.Environment {
	return clip.Environment{
		MediaLeaseTTL: cfg.ClipMediaLeaseTTL, MediaWaitTimeout: cfg.ClipMediaWaitTimeout,
		MediaStageTimeout: cfg.ClipMediaStageTimeout, MediaMaxAttempts: cfg.ClipMediaMaxAttempts,
		WorkRoot: cfg.ClipWorkRoot, FFmpegPath: cfg.ClipFFmpegPath, FFprobePath: cfg.ClipFFprobePath,
		ResvgPath: cfg.ClipResvgPath, OverlayDir: cfg.ClipOverlayDir, FontPaths: cfg.ClipFontPaths,
		WorkStaleAge: cfg.ClipWorkStaleAge, MediaTimeout: cfg.ClipMediaTimeout,
		EncodeThreads: cfg.ClipEncodeThreads, DecodeThreads: cfg.ClipDecodeThreads,
		SourceBatchTTL: cfg.ClipSourceBatchTTL, PutTTL: cfg.PresignPutTTL, GetTTL: cfg.PresignGetTTL,
		OrphanMinAge: cfg.OrphanMinAge, QuoteTTL: cfg.ClipQuoteTTL,
	}
}

func clipBudgets(cfg clipai.Config) clipapp.Budgets {
	return clipapp.Budgets{ObserveCompletionTokens: cfg.ObserveCompletionTokens, FlowCompletionTokens: cfg.FlowCompletionTokens, NarrationCompletionTokens: cfg.NarrationCompletionTokens, ObserveReasoning: cfg.ObserveReasoning, PlanReasoning: cfg.PlanReasoning}
}

func newClipGeneration(ctx context.Context, cfg *config.Config, store *clipstore.Store, projects *clipapp.Service, sources *clipapp.SourceService, bucket *storage.Bucket, media *clipmedia.Adapter, models meteredRegistry, queue *job.Queue, guard clipapp.Reserver, writer *sql.DB, bind clipapp.Binder) (*clipapp.GenerationService, error) {
	renderer, err := clipmedia.NewRenderer(media, clip.DefaultRenderConfig(clipEnvironment(cfg)))
	if err != nil {
		return nil, err
	}
	aiConfig := clipai.DefaultConfig(clipEnvironment(cfg))
	planner, err := clipai.New(clipModels{models}, renderer, aiConfig)
	if err != nil {
		return nil, err
	}
	finisher := clipapp.NewFinisher(writer, bind, jobstore.New(writer, writer, jobKinds()), store, nil)
	var remote *clipapp.MediaDispatch
	if cfg.ClipMediaExecution == "worker" {
		remote, err = clipapp.NewMediaDispatch(writer, bind, clip.DefaultMediaStageLimits(clipEnvironment(cfg)), clip.DefaultMediaConfig(clipEnvironment(cfg)), nil)
		if err != nil {
			return nil, err
		}
	}
	service := clipapp.NewGenerationService(store, projects, sources, bucket, media, planner, renderer, clipapp.NewJobs(queue, guard), clip.DefaultGenerationConfig(clipEnvironment(cfg)), clipapp.GenerationDeps{
		RemoteMedia: remote,
		Finisher:    finisher,
		Pricing:     clipapp.NewPricing(models.Registry, clipBudgets(aiConfig)),
		Accounting:  clipapp.NewAccounting(models.ledger),
		Admission:   clipapp.NewModelAdmission(models.Registry, clipBudgets(aiConfig)),
	})
	if _, err = queue.SweepUnactivated(ctx); err != nil {
		return nil, err
	}
	if err := finisher.Recover(ctx); err != nil {
		return nil, err
	}
	return service, nil
}

type clipModels struct{ meteredRegistry }

func (m clipModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return m.Lookup(ref) }
