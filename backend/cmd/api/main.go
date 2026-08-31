// Command api is the postpilot HTTP/RPC API server.
//
// This file is the composition root: the only place that wires configuration,
// infrastructure clients, and the server together. Every other package depends
// inward. The Connect server itself (mux, h2c, CORS, /health) is assembled in
// internal/platform/rpcserver.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/auth/provision"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/experiment"
	experimentrpc "github.com/postpilot/backend/internal/experiment/rpc"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/generation"
	generationrpc "github.com/postpilot/backend/internal/generation/rpc"
	"github.com/postpilot/backend/internal/health"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/llm/openaicompat"
	"github.com/postpilot/backend/internal/plan"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
	providerrpc "github.com/postpilot/backend/internal/provider/rpc"
	providerstore "github.com/postpilot/backend/internal/provider/store"
	"github.com/postpilot/backend/internal/publishing"
	publishingrpc "github.com/postpilot/backend/internal/publishing/rpc"
	publishingstore "github.com/postpilot/backend/internal/publishing/store"
	"github.com/postpilot/backend/internal/purpose"
	purposerpc "github.com/postpilot/backend/internal/purpose/rpc"
	purposestore "github.com/postpilot/backend/internal/purpose/store"
	"github.com/postpilot/backend/internal/storage"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// adapters is the set of provider protocols this binary ships (PRD §6.4: 필요할 때
// 하나씩). This is the only place an adapter package is imported — the composition
// root injects them into the port, and nothing above the port sees them.
var adapters = map[string]llm.AdapterFactory{
	"openai_compatible": openaicompat.Factory,
}

const version = "0.0.1"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// Subcommand dispatch keeps the production image at one binary and one ENTRYPOINT,
	// so `docker compose run --rm api adduser <id>` works against the deployed image
	// with nothing added to it.
	if len(os.Args) > 1 && os.Args[1] == "adduser" {
		if err := provision.Run(context.Background(), os.Args[2:], defaultVoiceBootstrap); err != nil {
			slog.Error("adduser failed", "err", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "setplan" {
		if err := provision.SetPlan(context.Background(), os.Args[2:]); err != nil {
			slog.Error("setplan failed", "err", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	// Same posture as a migration: a registry that does not validate must not serve.
	// A missing API key is NOT that — the provider's models come up disabled instead.
	// Checked before the database is touched: it is pure configuration, and a typo in
	// the yaml should not leave a half-started process behind.
	registry, err := llm.Load(cfg.ProvidersConfig, os.Getenv, adapters, llm.Options{
		Timeout:   cfg.LLMStageTimeout,
		MaxTokens: cfg.LLMMaxTokensDefault,
	})
	if err != nil {
		slog.Error("providers config invalid", "err", err)
		os.Exit(1)
	}
	for _, m := range registry.Models() {
		if m.Disabled {
			slog.Warn("model disabled", "model", m.Ref.String(), "reason", m.DisabledReason)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database open failed", "path", cfg.DBPath, "err", err)
		os.Exit(1)
	}
	defer handle.Close()

	// Migrations run before the listener exists, and a failure exits non-zero ([I7]).
	// That ordering is the whole rollback mechanism: the container never answers
	// /health, so the deploy gate restores the previous image (DEPLOY.md §2).
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	jobQueue := job.New(jobstore.New(handle.Writer, handle.Reader), config.WorkerPollInterval)
	if n, err := jobQueue.SweepRunning(ctx); err != nil {
		slog.Error("running job sweep failed", "err", err)
		os.Exit(1)
	} else if n > 0 {
		slog.Info("swept interrupted jobs", "count", n)
	}
	if n, err := jobQueue.SweepQueuedPersonalization(ctx); err != nil {
		slog.Error("queued personalization sweep failed", "err", err)
		os.Exit(1)
	} else if n > 0 {
		slog.Info("held queued personalization for explicit retry", "count", n)
	}

	authSvc := auth.NewService(authstore.New(handle.Writer, handle.Reader), cfg.SessionTTL)
	if n, err := authSvc.SweepExpired(ctx); err != nil {
		// Stale rows are harmless — they fail the expiry check on lookup anyway — so a
		// sweep failure is not worth refusing to serve over.
		slog.Warn("expired session sweep failed", "err", err)
	} else if n > 0 {
		slog.Info("swept expired sessions", "count", n)
	}

	// The ledger is built before any context that can spend: `metered` is the registry
	// every context is given from here on, so a call made anywhere lands on the ledger
	// without that context knowing the ledger exists.
	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), registry)
	meteredModels := meteredRegistry{Registry: registry, ledger: ledger}
	jobQueue.Admit(jobAdmission{ledger: ledger, registry: registry, plans: authSvc})

	// Checked here rather than in config.Load: `api adduser` must work on a fresh box
	// before a bucket exists. This still runs before the listener, so a missing value
	// keeps /health dark and the deploy rolls back.
	if err := cfg.RequireObjectStorage(); err != nil {
		slog.Error("object storage config invalid", "err", err)
		os.Exit(1)
	}

	bucket, err := storage.New(ctx, storage.Config{
		Endpoint:        cfg.R2Endpoint,
		PublicEndpoint:  cfg.R2PublicEndpoint,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket:          cfg.R2Bucket,
		MaxReadBytes:    cfg.MaxImageBytes,
	})
	if err != nil {
		slog.Error("object storage setup failed", "err", err)
		os.Exit(1)
	}

	postSvc := post.NewService(
		poststore.New(handle.Writer, handle.Reader),
		bucket,
		cfg.PresignPutTTL,
		cfg.PresignGetTTL,
		cfg.MaxImageBytes,
		postJobFinder{queue: jobQueue},
	)
	publishSvc := publishing.NewService(
		publishingstore.New(handle.Writer, handle.Reader),
		publishingPosts{service: postSvc},
		bucket,
		publishing.Config{
			PairingTTL: cfg.PublishPairingTTL, MaxPendingPairings: cfg.PublishMaxPendingPairings,
			LeaseTTL: cfg.PublishLeaseTTL, AssetURLTTL: cfg.PublishAssetURLTTL,
			AgentHeartbeatInterval: cfg.PublishAgentHeartbeatInterval,
		},
	)
	if requeued, unknown, err := publishSvc.RecoverExpired(ctx); err != nil {
		slog.Error("publishing recovery failed", "err", err)
		os.Exit(1)
	} else if requeued > 0 || unknown > 0 {
		slog.Info("publishing jobs recovered", "requeued", requeued, "outcome_unknown", unknown)
	}

	purposeSvc := purpose.NewService(
		purposestore.New(handle.Writer, handle.Reader),
		purpose.Limits{
			NameMaxChars: cfg.PurposeNameMaxChars, DescriptionMaxChars: cfg.PurposeDescriptionMaxChars,
			InstructionsMaxChars: cfg.PurposeInstructionsMaxChars,
		},
	)
	postSvc.SetPurposeDirectory(postPurposes{service: purposeSvc})

	providerSvc := provider.NewService(providerstore.New(handle.Writer, handle.Reader), meteredModels)
	voiceSvc := voice.NewService(
		voicestore.New(handle.Writer, handle.Reader),
		voiceModels{selections: providerSvc, registry: meteredModels, plans: authSvc},
		voiceJobs{queue: jobQueue},
	)
	voiceSvc.ConfigurePersonalization(voicePosts{service: postSvc}, voice.PersonalizationConfig{
		FewShotTargetCount: cfg.VoicePersonalization.FewShotTargetCount, FewShotMax: cfg.VoicePersonalization.FewShotMax,
		FewShotExcerptTargetChars: cfg.VoicePersonalization.FewShotExcerptTargetChars, FewShotExcerptMaxChars: cfg.VoicePersonalization.FewShotExcerptMaxChars,
		EmbeddingSwitchPosts: cfg.VoicePersonalization.EmbeddingSwitchPosts, DiffMaxRules: cfg.VoicePersonalization.DiffMaxRules,
		DiffMinPatternEdits: cfg.VoicePersonalization.DiffMinPatternEdits, RuleActivationEvidence: cfg.VoicePersonalization.RuleActivationEvidence,
		RuleRetireAfter: cfg.VoicePersonalization.RuleRetireAfter, ValidationPostCount: cfg.VoicePersonalization.ValidationPostCount,
		EndingMaxConsecutive: cfg.VoicePersonalization.EndingMaxConsecutive,
	})
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})
	jobQueue.Register(job.KindAnalyzeVoice, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.Analyze(ctx, voice.AnalysisJob{
			UserID: found.UserID, VoiceID: found.VoiceID, WriteModel: found.WriteModel,
		}, voice.Progress(progress))
	}))
	jobQueue.Register(job.KindLearnVoice, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.Learn(ctx, voice.LearningJob{UserID: found.UserID, EventID: strings.TrimSpace(string(found.Payload)), WriteModel: found.WriteModel}, voice.Progress(progress))
	}))
	jobQueue.Register(job.KindCompareVoiceRule, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.CompareRule(ctx, found.UserID, strings.TrimSpace(string(found.Payload)), found.WriteModel, voice.Progress(progress))
	}))
	jobQueue.Register(job.KindValidateVoiceProfile, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.ValidateProfile(ctx, found.UserID, strings.TrimSpace(string(found.Payload)), voice.Progress(progress))
	}))
	generationSvc := generation.NewService(
		generationPosts{service: postSvc},
		generationProfiles{service: voiceSvc},
		generationRules{service: voiceSvc},
		generationModels{registry: meteredModels},
		generationImages{bucket: bucket},
		generationJobs{queue: jobQueue},
		cfg.ObserveBatchSize,
		generation.ReasoningPolicy{Observe: cfg.LLMReasoning.Observe, Write: cfg.LLMReasoning.Write},
	)
	// Generation reads a brief only at enqueue, to freeze it; the purpose context never
	// learns that generation exists.
	generationSvc.SetPurposeBriefs(generationPurposes{service: purposeSvc})
	experimentStore := experimentstore.New(handle.Writer, handle.Reader)
	experimentSvc := experiment.NewService(
		experimentStore,
		experimentCatalog{selections: providerSvc, registry: meteredModels, plans: authSvc},
		experimentJobs{queue: jobQueue},
		experimentRunner{generation: generationSvc, voice: voiceSvc},
		cfg.ExperimentContentRetention,
	)
	if n, err := experimentSvc.RecoverInterrupted(ctx); err != nil {
		slog.Error("interrupted experiment recovery failed", "err", err)
		os.Exit(1)
	} else if n > 0 {
		slog.Info("recovered interrupted experiments", "count", n)
	}
	generationSvc.SetPendingExperimentFinder(postExperiments{service: experimentSvc})
	postSvc.SetPendingExperimentFinder(postExperiments{service: experimentSvc})
	postSvc.SetExperimentContentPurger(postExperiments{service: experimentSvc})
	// Voice and experiment guard each other through ports adapted only here: a voice with a
	// publishable experiment cannot be deleted, and an experiment cannot start or retry in a
	// deleted voice.
	voiceSvc.SetExperimentGuard(voiceExperiments{service: experimentSvc})
	experimentSvc.SetVoiceDirectory(experimentVoices{service: voiceSvc})
	jobQueue.Register(job.KindModelExperiment, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		experimentID := strings.TrimSpace(string(found.Payload))
		if experimentID == "" {
			return errors.New("model experiment payload is missing")
		}
		return experimentSvc.Handle(ctx, experimentID, experiment.Progress(progress))
	}))
	jobQueue.Register(job.KindGenerate, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		if found.PostSlug == nil {
			return job.ErrInvalidTarget
		}
		options, err := generation.DecodeGenerationPayload(found.Payload)
		if err != nil {
			return err
		}
		return generationSvc.Generate(ctx, generation.GenerateJob{
			UserID: found.UserID, PostSlug: *found.PostSlug, VoiceID: found.VoiceID,
			ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
			TargetLanguage: options.TargetLanguage, TargetLength: options.TargetLength, Purpose: options.Purpose,
		}, generation.Progress(progress))
	}))
	jobQueue.Register(job.KindRevise, metered(func(ctx context.Context, found job.Job, progress job.Progress) error {
		if found.PostSlug == nil {
			return job.ErrInvalidTarget
		}
		return generationSvc.Revise(ctx, generation.RevisionJob{
			UserID: found.UserID, PostSlug: *found.PostSlug, VoiceID: found.VoiceID, WriteModel: found.WriteModel,
			Payload: found.Payload,
		}, generation.Progress(progress))
	}))

	server := rpcserver.New(cfg, version, rpcserver.Options{
		Interceptors: []connect.Interceptor{authrpc.NewInterceptor(authSvc), publishingrpc.NewAgentInterceptor(publishSvc)},
		Handlers: []rpcserver.Registrar{
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewHealthServiceHandler(health.NewHandler(version), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(authSvc, cfg.SessionTTL), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewPlanServiceHandler(planrpc.NewHandler(ledger), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewAdminServiceHandler(authrpc.NewAdminHandler(authSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewPostServiceHandler(postrpc.NewHandler(postSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewProviderServiceHandler(providerrpc.NewHandler(providerSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewPurposeServiceHandler(purposerpc.NewHandler(purposeSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewGenerationServiceHandler(generationrpc.NewHandler(generationSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewVoiceServiceHandler(voicerpc.NewHandler(voiceSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewVoiceLearningServiceHandler(voicerpc.NewLearningHandler(voiceSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewVoiceValidationServiceHandler(voicerpc.NewValidationHandler(voiceSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewModelExperimentServiceHandler(experimentrpc.NewHandler(experimentSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewPublishingServiceHandler(publishingrpc.NewUserHandler(publishSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewPublishingAgentServiceHandler(publishingrpc.NewAgentHandler(publishSvc), opts...)
			},
		},
	})

	// The sweep starts with the server and stops with it. It is deliberately not run at
	// boot: a restart loop would turn every crash into a full bucket listing, and
	// nothing it collects is urgent.
	sweeper := post.NewSweeper(
		poststore.New(handle.Writer, handle.Reader),
		bucket,
		cfg.OrphanMinAge,
	)
	go sweeper.Run(ctx, cfg.OrphanSweepInterval)
	go experiment.NewSweeper(experimentStore).Run(ctx, cfg.ExperimentSweepInterval)
	go publishing.NewSweeper(publishSvc, cfg.PublishOrphanMinAge, cfg.PublishLeaseTTL).Run(ctx, cfg.PublishOrphanSweepInterval)

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	for range config.WorkerConcurrency {
		go jobQueue.Run(workerCtx)
	}

	// A serve error (e.g. the port is already bound) must NOT exit 0 — an orchestrator
	// would read a clean exit as success and never restart us. Surface it on a channel
	// so the startup path can exit non-zero.
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port, "version", version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-serveErr:
		slog.Error("listen failed", "err", err)
		os.Exit(1)
	}
	cancelWorkers()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "err", err)
	}
}

type voiceModels struct {
	selections *provider.Service
	registry   meteredRegistry
	plans      *auth.Service
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

func (a voiceModels) AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error) {
	acting, err := a.plans.PlanOf(ctx, userID)
	if err != nil {
		return llm.ModelRef{}, false, err
	}
	selections, err := a.selections.GetSelections(ctx, userID, acting)
	if err != nil {
		return llm.ModelRef{}, false, err
	}
	for _, selection := range selections {
		if selection.Stage != provider.StageAnalyze || selection.Missing {
			continue
		}
		info, ok := a.registry.Lookup(selection.Ref)
		if !ok || info.Disabled {
			return llm.ModelRef{}, false, nil
		}
		return selection.Ref, true, nil
	}
	return llm.ModelRef{}, false, nil
}

func (a voiceModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) { return a.registry.Lookup(ref) }

func (a voiceModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return a.registry.Complete(ctx, ref, request)
}

func (a voiceModels) ModelEnabled(ref llm.ModelRef) bool {
	info, ok := a.registry.Lookup(ref)
	return ok && !info.Disabled
}

type voiceJobs struct{ queue *job.Queue }

func (a voiceJobs) Enqueue(ctx context.Context, request voice.AnalysisJobRequest) (string, error) {
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindAnalyzeVoice, UserID: request.UserID, VoiceID: request.VoiceID, WriteModel: request.WriteModel,
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

func (a voiceJobs) EnqueuePersonalization(ctx context.Context, request voice.PersonalizationJobRequest) (string, error) {
	var postSlug *string
	if request.PostSlug != "" {
		value := request.PostSlug
		postSlug = &value
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: request.Kind, UserID: request.UserID, PostSlug: postSlug, VoiceID: request.VoiceID,
		WriteModel: request.Model, ExtraModels: request.ExtraModels, Payload: []byte(request.Payload),
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

func (a voiceJobs) IsPersonalizationJobActive(ctx context.Context, jobID, userID string) (bool, error) {
	if jobID == "" {
		return false, nil
	}
	found, err := a.queue.Get(ctx, jobID, userID)
	if errors.Is(err, job.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return found.Status == job.StatusQueued || found.Status == job.StatusRunning, nil
}

func (a voiceJobs) FailQueuedPersonalization(ctx context.Context, jobID, userID string, failure voice.Failure) (bool, error) {
	return a.queue.FailQueued(ctx, jobID, userID, job.Failure{Reason: failure.Reason, Params: failure.Params, TechnicalDetail: failure.TechnicalDetail})
}

func (a voiceJobs) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveForVoiceKind(ctx, voiceID, kind)
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
}

func (a voiceJobs) HasActiveForVoice(ctx context.Context, voiceID string) (bool, error) {
	return a.queue.HasActiveForVoice(ctx, voiceID)
}

// voiceExperiments adapts the experiment context's publishable-work guard for DeleteVoice.
type voiceExperiments struct{ service *experiment.Service }

func (a voiceExperiments) HasPublishableExperimentForVoice(ctx context.Context, userID, voiceID string) (bool, error) {
	return a.service.HasPublishableForVoice(ctx, userID, voiceID)
}

// postVoices adapts the voice directory for the post context: every owned voice, tombstones
// included, so a post keeps a name after its voice is deleted.
type postVoices struct{ service *voice.Service }

func (a postVoices) Voices(ctx context.Context, userID string) ([]post.VoiceRef, error) {
	voices, err := a.service.ListVoices(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]post.VoiceRef, 0, len(voices))
	for _, v := range voices {
		out = append(out, post.VoiceRef{ID: v.ID, Name: v.Name, Deleted: v.Deleted(), SourceLanguage: post.Language(v.SourceLanguage)})
	}
	return out, nil
}

// postPurposes adapts the purpose directory for the post context. Unlike voices there are no
// tombstones: a deleted purpose simply stops being listed, and the composite foreign key has
// already cleared the assignments that named it.
type postPurposes struct{ service *purpose.Service }

func (a postPurposes) Purposes(ctx context.Context, userID string) ([]post.PurposeRef, error) {
	purposes, err := a.service.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]post.PurposeRef, 0, len(purposes))
	for _, p := range purposes {
		out = append(out, post.PurposeRef{ID: p.ID, Name: p.Name})
	}
	return out, nil
}

// generationPurposes hands the generation context the frozen text of one owned purpose. It is
// consulted once per enqueue; no handler ever calls it.
type generationPurposes struct{ service *purpose.Service }

func (a generationPurposes) BriefFor(ctx context.Context, userID, purposeID string) (generation.PurposeBrief, bool, error) {
	brief, ok, err := a.service.BriefFor(ctx, userID, purposeID)
	if err != nil || !ok {
		return generation.PurposeBrief{}, false, err
	}
	return generation.PurposeBrief{Name: brief.Name, Description: brief.Description, Instructions: brief.Instructions}, true, nil
}

// experimentVoices adapts the directory for the experiment context: only an owned, active
// voice may start or retry a comparison in its name.
type experimentVoices struct{ service *voice.Service }

func (a experimentVoices) ActiveVoice(ctx context.Context, userID, voiceID string) error {
	found, err := a.service.GetVoice(ctx, userID, voiceID)
	switch {
	case errors.Is(err, voice.ErrVoiceNotFound), errors.Is(err, voice.ErrVoiceRequired):
		return experiment.ErrVoiceUnavailable
	case err != nil:
		return err
	case found.Deleted():
		return experiment.ErrVoiceUnavailable
	}
	return nil
}

type postJobFinder struct {
	queue *job.Queue
}

type voicePosts struct{ service *post.Service }

func (a voicePosts) LearningSnapshot(ctx context.Context, userID, slug string) (voice.FinalizationInput, error) {
	found, err := a.service.LearningSnapshot(ctx, userID, slug)
	if err != nil {
		switch {
		case errors.Is(err, post.ErrNotFound):
			return voice.FinalizationInput{}, voice.ErrPostNotFound
		case errors.Is(err, post.ErrForbidden):
			return voice.FinalizationInput{}, voice.ErrForbidden
		case errors.Is(err, post.ErrNoMachineBaseline), errors.Is(err, post.ErrPostNotFinalized):
			return voice.FinalizationInput{}, voice.ErrInvalidLifecycle
		default:
			return voice.FinalizationInput{}, err
		}
	}
	baseline, err := json.Marshal(postContentWire(found.MachineBaseline))
	if err != nil {
		return voice.FinalizationInput{}, err
	}
	current, err := json.Marshal(postContentWire(found.Current))
	if err != nil {
		return voice.FinalizationInput{}, err
	}
	return voice.FinalizationInput{PostSlug: found.PostSlug, UserID: found.UserID, VoiceID: found.VoiceID, BaselineVoiceID: found.MachineBaselineVoiceID, BaselineJSON: string(baseline), FinalJSON: string(current), Title: found.Current.Title, Tags: found.Current.Tags, BaselineRevision: found.BaselineRevision, ContentRevision: found.ContentRevision, TargetLength: found.TargetLength, ContentLanguage: voice.Language(found.ContentLanguage), VoiceSourceLanguage: voice.Language(found.VoiceSourceLanguage)}, nil
}

type postBlockWire struct {
	Type    string   `json:"type"`
	Content string   `json:"content,omitempty"`
	Level   int32    `json:"level,omitempty"`
	File    string   `json:"file,omitempty"`
	Alt     string   `json:"alt,omitempty"`
	Caption string   `json:"caption,omitempty"`
	Items   []string `json:"items,omitempty"`
}
type postContentJSONWire struct {
	Title   string          `json:"title"`
	Summary string          `json:"summary"`
	Tags    []string        `json:"tags"`
	Blocks  []postBlockWire `json:"blocks"`
}

func postContentWire(content post.PostContent) postContentJSONWire {
	out := postContentJSONWire{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		out.Blocks = append(out.Blocks, postBlockWire{Type: string(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
	}
	return out
}

type generationModels struct{ registry meteredRegistry }

type generationImages struct{ bucket *storage.Bucket }

func (a generationImages) Read(ctx context.Context, key string) ([]byte, error) {
	return a.bucket.ReadObject(ctx, key)
}

func (a generationModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return a.registry.Lookup(ref)
}

func (a generationModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return a.registry.Complete(ctx, ref, request)
}

type generationProfiles struct{ service *voice.Service }

func (a generationProfiles) ProfileForPrompt(ctx context.Context, userID, voiceID string, target generation.Language) (generation.Profile, error) {
	return a.ProfileForPromptForTopic(ctx, userID, voiceID, target, "", nil)
}

func (a generationProfiles) ProfileForPromptForTopic(ctx context.Context, userID, voiceID string, target generation.Language, topic string, tags []string) (generation.Profile, error) {
	profile, err := a.service.PromptProfileForTopicAndLanguage(ctx, userID, voiceID, voice.Language(target), topic, tags)
	return generation.Profile{Styleguide: profile.Styleguide, ActiveRules: profile.ActiveRules, Excerpts: profile.Excerpts, Rules: profile.ManualRules, EndingMaxConsecutive: a.service.EndingMaxConsecutive(), SourceLanguage: generation.Language(profile.SourceLanguage), TargetLanguage: generation.Language(profile.TargetLanguage), Portable: profile.Portable}, generationVoiceError(err)
}

type generationRules struct{ service *voice.Service }

func (a generationRules) AppendRule(ctx context.Context, userID, voiceID, line string) error {
	return generationVoiceError(a.service.AppendRule(ctx, userID, voiceID, line))
}

func generationVoiceError(err error) error {
	switch {
	case errors.Is(err, voice.ErrVoiceDeleted):
		return generation.ErrVoiceDeleted
	case errors.Is(err, voice.ErrVoiceNotFound), errors.Is(err, voice.ErrVoiceRequired):
		return generation.ErrVoiceRequired
	default:
		return err
	}
}

type generationPosts struct{ service *post.Service }

func (a generationPosts) AttachedImages(ctx context.Context, userID, slug string) (generation.PostInput, error) {
	found, err := a.service.AttachedImages(ctx, userID, slug)
	if err != nil {
		return generation.PostInput{}, generationPostError(err)
	}
	input := generation.PostInput{
		Slug: found.Slug, UserID: found.UserID, Title: found.Title, Memo: found.Memo,
		Voice:          generation.VoiceRef{ID: found.Voice.ID, Name: found.Voice.Name, Deleted: found.Voice.Deleted, SourceLanguage: generation.Language(found.Voice.SourceLanguage)},
		TargetLanguage: generation.Language(found.TargetLanguage),
		// The id, never the brief: only the enqueue resolves it, and only through the purpose
		// context's own port. Dropping it here is what would make the whole feature a silent
		// no-op — every prompt would be built as if no post ever had a 용도.
		PurposeID:    found.PurposeID,
		TargetLength: found.TargetLength,
		Images:       make([]generation.Image, 0, len(found.Images)),
	}
	if found.ContentLanguage != nil {
		contentLanguage := generation.Language(*found.ContentLanguage)
		input.ContentLanguage = &contentLanguage
	}
	if found.Content != nil {
		content := generation.PostContent{
			Title: found.Content.Title, Summary: found.Content.Summary, Tags: found.Content.Tags,
		}
		for _, block := range found.Content.Blocks {
			content.Blocks = append(content.Blocks, generation.Block{
				Type: generation.BlockType(block.Type), Content: block.Content, Level: block.Level,
				File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
			})
		}
		input.Content = &content
	}
	for _, image := range found.Images {
		input.Images = append(input.Images, generation.Image{Filename: image.Filename, Key: image.Key})
	}
	return input, nil
}

func (a generationPosts) SetObservations(ctx context.Context, userID, slug string, observations []generation.Observation) error {
	values := make([]post.Observation, 0, len(observations))
	for _, observation := range observations {
		values = append(values, post.Observation{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent,
		})
	}
	return generationPostError(a.service.SetObservations(ctx, userID, slug, values))
}

func (a generationPosts) SetGeneratedContent(ctx context.Context, userID, slug string, content generation.PostContent, language generation.Language) error {
	value := post.PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		value.Blocks = append(value.Blocks, post.Block{
			Type: post.BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return generationPostError(a.service.SetGeneratedContent(ctx, userID, slug, value, post.Language(language)))
}

func generationPostError(err error) error {
	switch {
	case errors.Is(err, post.ErrNotFound):
		return generation.ErrNotFound
	case errors.Is(err, post.ErrForbidden):
		return generation.ErrForbidden
	default:
		return err
	}
}

type generationJobs struct{ queue *job.Queue }

func (a generationJobs) EnqueueGeneration(ctx context.Context, request generation.StartRequest) (string, error) {
	slug := request.PostSlug
	payload, err := generation.EncodeGenerationPayload(generation.GenerationOptions{
		TargetLanguage: request.TargetLanguage, TargetLength: request.TargetLength, Purpose: request.Purpose,
	})
	if err != nil {
		return "", err
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindGenerate, UserID: request.UserID, PostSlug: &slug, VoiceID: request.VoiceID,
		ObserveModel: request.ObserveModel, WriteModel: request.WriteModel,
		TargetLanguage: request.TargetLanguage.String(), Payload: payload,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &generation.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", generation.ErrVoiceDeleted
	}
	return id, err
}

func (a generationJobs) EnqueueRevision(ctx context.Context, request generation.StartRevisionRequest, payload []byte) (string, error) {
	slug := request.PostSlug
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindRevise, UserID: request.UserID, PostSlug: &slug, VoiceID: request.VoiceID,
		WriteModel: request.WriteModel, TargetLanguage: request.ContentLanguage.String(), Payload: payload,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &generation.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", generation.ErrVoiceDeleted
	}
	return id, err
}

func (a generationJobs) GetGeneration(ctx context.Context, id, userID string) (*generation.JobSummary, error) {
	found, err := a.queue.Get(ctx, id, userID)
	if err != nil {
		switch {
		case errors.Is(err, job.ErrNotFound):
			return nil, generation.ErrNotFound
		case errors.Is(err, job.ErrForbidden):
			return nil, generation.ErrForbidden
		default:
			return nil, err
		}
	}
	postSlug := ""
	if found.PostSlug != nil {
		postSlug = *found.PostSlug
	}
	return &generation.JobSummary{
		ID: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage,
		ProgressDone: found.ProgressDone, ProgressTotal: found.ProgressTotal,
		Failure:  generationFailure(found.Failure),
		PostSlug: postSlug, ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		TargetLanguage: generation.Language(found.TargetLanguage),
		CreatedAt:      found.CreatedAt, UpdatedAt: found.UpdatedAt,
	}, nil
}

func (a postJobFinder) ActiveForPost(ctx context.Context, slug string) (*post.ActiveJob, error) {
	found, err := a.queue.ActiveForPost(ctx, slug)
	if err != nil || found == nil {
		return nil, err
	}
	postSlug := ""
	if found.PostSlug != nil {
		postSlug = *found.PostSlug
	}
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

type experimentJobs struct{ queue *job.Queue }

func (a experimentJobs) EnqueueExperiment(ctx context.Context, request experiment.JobRequest) (string, error) {
	var postSlug *string
	if request.PostSlug != "" {
		value := request.PostSlug
		postSlug = &value
	}
	targetLanguage := ""
	if request.TargetLanguage != nil {
		targetLanguage = request.TargetLanguage.String()
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindModelExperiment, UserID: request.UserID, PostSlug: postSlug, VoiceID: request.VoiceID,
		TargetLanguage: targetLanguage, Payload: []byte(request.ExperimentID), ExtraModels: request.Models,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &experiment.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", experiment.ErrVoiceUnavailable
	}
	return id, err
}

func (a experimentJobs) HasRunnableExperiment(ctx context.Context, experimentID string) (bool, error) {
	return a.queue.HasRunnableExperiment(ctx, experimentID)
}

type postExperiments struct{ service *experiment.Service }

func (a postExperiments) PendingForPost(ctx context.Context, userID, slug string) (string, error) {
	found, err := a.service.PendingForPost(ctx, userID, slug)
	if err != nil || found == nil {
		return "", err
	}
	return found.ID, nil
}

func (a postExperiments) PurgePost(ctx context.Context, userID, slug string) error {
	return a.service.PurgePost(ctx, userID, slug)
}

type experimentCatalog struct {
	selections *provider.Service
	registry   meteredRegistry
	plans      *auth.Service
}

func (a experimentCatalog) Resolve(ref experiment.ModelRef) (experiment.Model, bool) {
	info, ok := a.registry.Lookup(llmRef(ref))
	if !ok {
		return experiment.Model{}, false
	}
	return experiment.Model{
		Ref: ref, Label: info.Label, Vision: info.Vision, Enabled: !info.Disabled,
		InputUSDPerMillion: info.InputUSDPerMillion, OutputUSDPerMillion: info.OutputUSDPerMillion,
	}, true
}

func (a experimentCatalog) Adopt(ctx context.Context, userID string, stage experiment.Stage, ref experiment.ModelRef) error {
	acting, err := a.plans.PlanOf(ctx, userID)
	if err != nil {
		return err
	}
	_, err = a.selections.SaveSelection(ctx, userID, acting, provider.Stage(stage), llmRef(ref))
	return err
}

func (a experimentCatalog) Active(ctx context.Context, userID string, stage experiment.Stage) (experiment.ModelRef, bool, error) {
	acting, err := a.plans.PlanOf(ctx, userID)
	if err != nil {
		return experiment.ModelRef{}, false, err
	}
	selections, err := a.selections.GetSelections(ctx, userID, acting)
	if err != nil {
		return experiment.ModelRef{}, false, err
	}
	for _, selection := range selections {
		if string(selection.Stage) == string(stage) && !selection.Missing {
			return experiment.ModelRef{ProviderID: selection.Ref.ProviderID, ModelID: selection.Ref.ModelID}, true, nil
		}
	}
	return experiment.ModelRef{}, false, nil
}

func (a experimentCatalog) Recommended(stage experiment.Stage, ref experiment.ModelRef) bool {
	for _, set := range a.registry.RecommendationSets() {
		for _, selection := range set.Selections {
			if selection.Stage != string(stage) {
				continue
			}
			candidate := llmRef(ref)
			if selection.Active == candidate || selection.CandidateA == candidate || selection.CandidateB == candidate {
				return true
			}
		}
	}
	return false
}

type experimentRunner struct {
	generation *generation.Service
	voice      *voice.Service
}

func (a experimentRunner) Snapshot(ctx context.Context, request experiment.StartRequest) (experiment.Snapshot, error) {
	switch request.Stage {
	case experiment.StageWrite:
		content, err := a.generation.SnapshotWriteInput(ctx, request.UserID, request.PostSlug, llmRef(request.ObserveModel), request.TargetLength)
		targetLanguage := experiment.Language(generation.SnapshotTargetLanguage(content))
		var frozenTarget *experiment.Language
		if targetLanguage.Valid() {
			frozenTarget = &targetLanguage
		}
		return experiment.Snapshot{
			Content: content, PromptVersion: generation.WriteExperimentPromptVersion,
			VoiceID: generation.SnapshotVoice(content), PurposeName: generation.SnapshotPurposeName(content), TargetLanguage: frozenTarget,
		}, mapSnapshotError(err)
	case experiment.StageObserve:
		content, err := a.generation.SnapshotObserveInput(ctx, request.UserID, request.PostSlug)
		return experiment.Snapshot{Content: content, PromptVersion: generation.ObserveExperimentPromptVersion}, mapSnapshotError(err)
	case experiment.StageAnalyze:
		content, err := a.voice.SnapshotAnalysisInput(ctx, request.UserID, request.VoiceID)
		return experiment.Snapshot{Content: content, PromptVersion: voice.AnalyzeExperimentPromptVersion, VoiceID: request.VoiceID}, experimentVoiceError(err)
	default:
		return experiment.Snapshot{}, experiment.ErrInvalidStage
	}
}

func (a experimentRunner) PrepareWrite(ctx context.Context, found experiment.Experiment, progress experiment.Progress) (experiment.Snapshot, error) {
	content, err := a.generation.PrepareWriteInput(ctx, found.InputSnapshot, generation.Progress(progress))
	return experiment.Snapshot{Content: content, PromptVersion: generation.WriteExperimentPromptVersion, VoiceID: found.VoiceID, PurposeName: found.PurposeName, TargetLanguage: cloneExperimentLanguage(found.TargetLanguage)}, mapSnapshotError(err)
}

func cloneExperimentLanguage(value *experiment.Language) *experiment.Language {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (a experimentRunner) RunCandidate(ctx context.Context, found experiment.Experiment, candidate experiment.Candidate, progress experiment.Progress) (experiment.CandidateResult, error) {
	ref := llmRef(candidate.Model)
	switch found.Stage {
	case experiment.StageWrite:
		content, usage, err := a.generation.RunWriteCandidate(ctx, found.InputSnapshot, ref)
		encoded, encodeErr := json.Marshal(toOutputPost(content))
		if err == nil {
			err = encodeErr
		}
		return experiment.CandidateResult{Output: encoded, Usage: experimentUsage(usage.PromptTokens, usage.CompletionTokens, usage.CostMicrousd, usage.CostReported)}, err
	case experiment.StageObserve:
		// Candidate progress is emitted once by the experiment coordinator. Forwarding
		// each vision pipeline's batch progress would race two incompatible counters.
		observations, usage, err := a.generation.RunObserveCandidate(ctx, found.InputSnapshot, ref, func(string, int, int) {})
		encoded, encodeErr := json.Marshal(toOutputObservations(observations))
		if err == nil {
			err = encodeErr
		}
		return experiment.CandidateResult{Output: encoded, Usage: experimentUsage(usage.PromptTokens, usage.CompletionTokens, usage.CostMicrousd, usage.CostReported)}, mapSnapshotError(err)
	case experiment.StageAnalyze:
		styleguide, usage, err := a.voice.RunAnalyzeCandidate(ctx, found.InputSnapshot, ref)
		encoded, encodeErr := json.Marshal(styleguide)
		if err == nil {
			err = encodeErr
		}
		return experiment.CandidateResult{Output: encoded, Usage: experimentUsage(usage.PromptTokens, usage.CompletionTokens, usage.CostMicrousd, usage.CostReported)}, err
	default:
		return experiment.CandidateResult{}, experiment.ErrInvalidStage
	}
}

func (a experimentRunner) ApplyWinner(ctx context.Context, found experiment.Experiment, candidate experiment.Candidate, confirmStyleguide bool) error {
	switch found.Stage {
	case experiment.StageWrite:
		var value outputPost
		if err := json.Unmarshal(candidate.Output, &value); err != nil {
			return fmt.Errorf("decode write winner: %w", err)
		}
		return mapSnapshotError(a.generation.ApplyWriteWinner(ctx, found.UserID, found.PostSlug, fromOutputPost(value), found.InputSnapshot))
	case experiment.StageObserve:
		var values []outputObservation
		if err := json.Unmarshal(candidate.Output, &values); err != nil {
			return fmt.Errorf("decode observation winner: %w", err)
		}
		return a.generation.ApplyObservationWinner(ctx, found.UserID, found.PostSlug, fromOutputObservations(values))
	case experiment.StageAnalyze:
		if !confirmStyleguide {
			return experiment.ErrConfirmationRequired
		}
		var styleguide string
		if err := json.Unmarshal(candidate.Output, &styleguide); err != nil {
			return fmt.Errorf("decode styleguide winner: %w", err)
		}
		return experimentVoiceError(a.voice.ApplyStyleguideWinner(ctx, found.UserID, found.VoiceID, styleguide))
	default:
		return experiment.ErrInvalidStage
	}
}

type outputPost struct {
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Tags    []string      `json:"tags"`
	Blocks  []outputBlock `json:"blocks"`
}
type outputBlock struct {
	Type    string   `json:"type"`
	Content string   `json:"content"`
	Level   int32    `json:"level"`
	File    string   `json:"file"`
	Alt     string   `json:"alt"`
	Caption string   `json:"caption"`
	Items   []string `json:"items"`
}
type outputObservation struct {
	File          string   `json:"file"`
	Scene         string   `json:"scene"`
	Mood          string   `json:"mood"`
	VisibleText   string   `json:"visible_text"`
	Objects       []string `json:"objects"`
	PeoplePresent bool     `json:"people_present"`
}

func toOutputPost(content generation.PostContent) outputPost {
	out := outputPost{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		out.Blocks = append(out.Blocks, outputBlock{Type: string(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
	}
	return out
}
func fromOutputPost(value outputPost) generation.PostContent {
	out := generation.PostContent{Title: value.Title, Summary: value.Summary, Tags: value.Tags}
	for _, block := range value.Blocks {
		out.Blocks = append(out.Blocks, generation.Block{Type: generation.BlockType(block.Type), Content: block.Content, Level: block.Level, File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items})
	}
	return out
}
func toOutputObservations(values []generation.Observation) []outputObservation {
	out := make([]outputObservation, 0, len(values))
	for _, value := range values {
		out = append(out, outputObservation{File: value.File, Scene: value.Scene, Mood: value.Mood, VisibleText: value.VisibleText, Objects: value.Objects, PeoplePresent: value.PeoplePresent})
	}
	return out
}
func fromOutputObservations(values []outputObservation) []generation.Observation {
	out := make([]generation.Observation, 0, len(values))
	for _, value := range values {
		out = append(out, generation.Observation{File: value.File, Scene: value.Scene, Mood: value.Mood, VisibleText: value.VisibleText, Objects: value.Objects, PeoplePresent: value.PeoplePresent})
	}
	return out
}
func experimentUsage(prompt, completion, cost int64, reported bool) experiment.UsageReport {
	return experiment.UsageReport{PromptTokens: prompt, CompletionTokens: completion, CostMicrousd: cost, CostReported: reported}
}
func experimentRef(value string) experiment.ModelRef {
	ref := parseLLMRef(value)
	return experiment.ModelRef{ProviderID: ref.ProviderID, ModelID: ref.ModelID}
}
func llmRef(ref experiment.ModelRef) llm.ModelRef {
	return llm.ModelRef{ProviderID: ref.ProviderID, ModelID: ref.ModelID}
}
func parseLLMRef(value string) llm.ModelRef {
	providerID, modelID, _ := strings.Cut(value, "/")
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}
}
func mapSnapshotError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, generation.ErrVoiceDeleted), errors.Is(err, generation.ErrVoiceMismatch), errors.Is(err, generation.ErrVoiceRequired):
		return experiment.ErrVoiceUnavailable
	case strings.Contains(err.Error(), "read photo"):
		return experiment.ErrSnapshotUnavailable
	}
	return err
}

func experimentVoiceError(err error) error {
	switch {
	case errors.Is(err, voice.ErrVoiceDeleted), errors.Is(err, voice.ErrVoiceNotFound), errors.Is(err, voice.ErrVoiceRequired):
		return experiment.ErrVoiceUnavailable
	default:
		return err
	}
}

// defaultVoiceBootstrap gives a freshly provisioned account its `기본 말투` before it can
// create a post. It is idempotent, so `adduser` may be rerun to repair an account that was
// left without a voice; a failure exits non-zero because the invariant is not established.
func defaultVoiceBootstrap(ctx context.Context, handle *db.DB, userID string) error {
	directory := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	_, _, err := directory.EnsureDefaultVoice(ctx, userID, voice.LanguageKorean)
	return err
}

var _ experiment.Runner = experimentRunner{}

// meteredRegistry is the model registry every context above the llm port is given. It is
// the registry, unchanged, except that each completed call is written to the account
// ledger — so "every server-side LLM call is metered" holds by construction rather than by
// each caller remembering to record one.
type meteredRegistry struct {
	*llm.Registry
	ledger *usage.Service
}

func (m meteredRegistry) Complete(ctx context.Context, ref llm.ModelRef, req llm.Request) (llm.Response, error) {
	response, err := m.Registry.Complete(ctx, ref, req)
	// A ledger failure never fails the user's work: the tokens are already spent, and the
	// budget it protects is a soft cap enforced at the NEXT admission.
	if recordErr := m.ledger.RecordCall(ctx, ref, response.Usage, err != nil); recordErr != nil {
		slog.Error("usage ledger write failed", "model", ref.String(), "err", recordErr)
	}
	return response, err
}

// jobAdmission is the plan gate at the job-enqueue seam. It is the one place that joins
// the three things the decision needs and that no single context holds: the acting plan
// (from the request context), the floors of the refs the job will run (from the registry),
// and the account's ledger position (from usage).
type jobAdmission struct {
	ledger   *usage.Service
	registry *llm.Registry
	plans    *auth.Service
}

func (a jobAdmission) Admit(ctx context.Context, start job.Start) error {
	// The request's own tier is preferred so one request is judged against one tier
	// throughout; a start made from a worker context has no session to read, and falls back
	// to the stored row, which is the same authority the interceptor resolved from.
	acting, ok := auth.PlanFromContext(ctx)
	if !ok {
		stored, err := a.plans.PlanOf(ctx, start.UserID)
		if err != nil {
			return fmt.Errorf("admit %s: resolve acting plan: %w", start.Kind, err)
		}
		acting = stored
	}
	floors := make([]plan.ModelFloor, 0, len(start.Models))
	for _, ref := range start.Models {
		info, found := a.registry.Lookup(parseRegistryRef(ref))
		if !found {
			// An unregistered ref is the enqueuing service's own failure to report, not a
			// plan refusal; it has already been validated there.
			continue
		}
		floors = append(floors, plan.ModelFloor{Ref: ref, MinPlan: info.MinPlan})
	}
	return a.ledger.Admit(ctx, usage.Start{
		UserID: start.UserID, Plan: acting, Kind: start.Kind, JobID: start.JobID, Models: floors,
	})
}

func (a jobAdmission) Release(ctx context.Context, jobID string) {
	if err := a.ledger.Release(ctx, jobID); err != nil {
		slog.Error("release admission failed", "job", jobID, "err", err)
	}
}

// parseRegistryRef splits the stored "provider/model" form a job records. The model id may
// itself contain slashes (`anthropic/claude-opus-5` under the `openrouter` provider), so
// only the first separator is a boundary.
func parseRegistryRef(ref string) llm.ModelRef {
	providerID, modelID, ok := strings.Cut(ref, "/")
	if !ok {
		return llm.ModelRef{}
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}
}

// metered attributes every provider call a job handler makes to the account and the job
// that caused it. Stamped once here, at the worker seam, so no handler below has to carry
// the ledger's identity through its own call graph.
func metered(handler job.Handler) job.Handler {
	return func(ctx context.Context, found job.Job, progress job.Progress) error {
		return handler(usage.WithWork(ctx, usage.Work{
			UserID: found.UserID, Kind: found.Kind, JobID: found.ID,
			ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		}), found, progress)
	}
}
