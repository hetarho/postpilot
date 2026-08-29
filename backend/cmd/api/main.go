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
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
	providerrpc "github.com/postpilot/backend/internal/provider/rpc"
	providerstore "github.com/postpilot/backend/internal/provider/store"
	"github.com/postpilot/backend/internal/storage"
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
		if err := provision.Run(context.Background(), os.Args[2:]); err != nil {
			slog.Error("adduser failed", "err", err)
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

	providerSvc := provider.NewService(providerstore.New(handle.Writer, handle.Reader), registry)
	voiceSvc := voice.NewService(
		voicestore.New(handle.Writer, handle.Reader),
		voiceModels{selections: providerSvc, registry: registry},
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
	jobQueue.Register(job.KindAnalyzeVoice, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.Analyze(ctx, voice.AnalysisJob{
			UserID: found.UserID, WriteModel: found.WriteModel,
		}, voice.Progress(progress))
	})
	jobQueue.Register(job.KindLearnVoice, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.Learn(ctx, voice.LearningJob{UserID: found.UserID, EventID: strings.TrimSpace(string(found.Payload)), WriteModel: found.WriteModel}, voice.Progress(progress))
	})
	jobQueue.Register(job.KindCompareVoiceRule, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.CompareRule(ctx, found.UserID, strings.TrimSpace(string(found.Payload)), found.WriteModel, voice.Progress(progress))
	})
	jobQueue.Register(job.KindValidateVoiceProfile, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return voiceSvc.ValidateProfile(ctx, found.UserID, strings.TrimSpace(string(found.Payload)), voice.Progress(progress))
	})
	generationSvc := generation.NewService(
		generationPosts{service: postSvc},
		generationProfiles{service: voiceSvc},
		generationRules{service: voiceSvc},
		generationModels{registry: registry},
		generationImages{bucket: bucket},
		generationJobs{queue: jobQueue},
		cfg.ObserveBatchSize,
	)
	experimentStore := experimentstore.New(handle.Writer, handle.Reader)
	experimentSvc := experiment.NewService(
		experimentStore,
		experimentCatalog{selections: providerSvc, registry: registry},
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
	jobQueue.Register(job.KindModelExperiment, func(ctx context.Context, found job.Job, progress job.Progress) error {
		experimentID := strings.TrimSpace(string(found.Payload))
		if experimentID == "" {
			return errors.New("model experiment payload is missing")
		}
		return experimentSvc.Handle(ctx, experimentID, experiment.Progress(progress))
	})
	jobQueue.Register(job.KindGenerate, func(ctx context.Context, found job.Job, progress job.Progress) error {
		if found.PostSlug == nil {
			return job.ErrInvalidTarget
		}
		targetLength, err := generation.DecodeGenerationPayload(found.Payload)
		if err != nil {
			return err
		}
		return generationSvc.Generate(ctx, generation.GenerateJob{
			UserID: found.UserID, PostSlug: *found.PostSlug,
			ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
			TargetLength: targetLength,
		}, generation.Progress(progress))
	})
	jobQueue.Register(job.KindRevise, func(ctx context.Context, found job.Job, progress job.Progress) error {
		if found.PostSlug == nil {
			return job.ErrInvalidTarget
		}
		return generationSvc.Revise(ctx, generation.RevisionJob{
			UserID: found.UserID, PostSlug: *found.PostSlug, WriteModel: found.WriteModel,
			Payload: found.Payload,
		}, generation.Progress(progress))
	})

	server := rpcserver.New(cfg, version, rpcserver.Options{
		Interceptors: []connect.Interceptor{authrpc.NewInterceptor(authSvc)},
		Handlers: []rpcserver.Registrar{
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewHealthServiceHandler(health.NewHandler(version), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(authSvc, cfg.SessionTTL), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewPostServiceHandler(postrpc.NewHandler(postSvc), opts...)
			},
			func(opts ...connect.HandlerOption) (string, http.Handler) {
				return postpilotv1connect.NewProviderServiceHandler(providerrpc.NewHandler(providerSvc), opts...)
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
	registry   *llm.Registry
}

func (a voiceModels) AnalyzeModel(ctx context.Context, userID string) (llm.ModelRef, bool, error) {
	selections, err := a.selections.GetSelections(ctx, userID)
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
		Kind: job.KindAnalyzeVoice, UserID: request.UserID, WriteModel: request.WriteModel,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	return id, err
}

func (a voiceJobs) EnqueuePersonalization(ctx context.Context, request voice.PersonalizationJobRequest) (string, error) {
	var postSlug *string
	if request.PostSlug != "" {
		value := request.PostSlug
		postSlug = &value
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{Kind: request.Kind, UserID: request.UserID, PostSlug: postSlug, WriteModel: request.Model, Payload: []byte(request.Payload)})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
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

func (a voiceJobs) FailQueuedPersonalization(ctx context.Context, jobID, userID, message string) (bool, error) {
	return a.queue.FailQueued(ctx, jobID, userID, message)
}

func (a voiceJobs) ActiveForUserKind(ctx context.Context, userID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveForUserKind(ctx, userID, kind)
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
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
	return voice.FinalizationInput{PostSlug: found.PostSlug, UserID: found.UserID, BaselineJSON: string(baseline), FinalJSON: string(current), Title: found.Current.Title, Tags: found.Current.Tags, BaselineRevision: found.BaselineRevision, ContentRevision: found.ContentRevision, TargetLength: found.TargetLength}, nil
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

type generationModels struct{ registry *llm.Registry }

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

func (a generationProfiles) ProfileForPrompt(ctx context.Context, userID string) (generation.Profile, error) {
	return a.ProfileForPromptForTopic(ctx, userID, "", nil)
}

func (a generationProfiles) ProfileForPromptForTopic(ctx context.Context, userID, topic string, tags []string) (generation.Profile, error) {
	profile, err := a.service.PromptProfileForTopic(ctx, userID, topic, tags)
	return generation.Profile{Styleguide: profile.Styleguide, ActiveRules: profile.ActiveRules, Excerpts: profile.Excerpts, Rules: profile.ManualRules, EndingMaxConsecutive: a.service.EndingMaxConsecutive()}, err
}

type generationRules struct{ service *voice.Service }

func (a generationRules) AppendRule(ctx context.Context, userID, line string) error {
	return a.service.AppendRule(ctx, userID, line)
}

type generationPosts struct{ service *post.Service }

func (a generationPosts) AttachedImages(ctx context.Context, userID, slug string) (generation.PostInput, error) {
	found, err := a.service.AttachedImages(ctx, userID, slug)
	if err != nil {
		return generation.PostInput{}, generationPostError(err)
	}
	input := generation.PostInput{
		Slug: found.Slug, UserID: found.UserID, Title: found.Title, Memo: found.Memo,
		TargetLength: found.TargetLength,
		Images:       make([]generation.Image, 0, len(found.Images)),
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

func (a generationPosts) SetGeneratedContent(ctx context.Context, userID, slug string, content generation.PostContent) error {
	value := post.PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		value.Blocks = append(value.Blocks, post.Block{
			Type: post.BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return generationPostError(a.service.SetGeneratedContent(ctx, userID, slug, value))
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
	payload, err := generation.EncodeGenerationPayload(request.TargetLength)
	if err != nil {
		return "", err
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindGenerate, UserID: request.UserID, PostSlug: &slug,
		ObserveModel: request.ObserveModel, WriteModel: request.WriteModel,
		Payload: payload,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &generation.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	return id, err
}

func (a generationJobs) EnqueueRevision(ctx context.Context, request generation.StartRevisionRequest, payload []byte) (string, error) {
	slug := request.PostSlug
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindRevise, UserID: request.UserID, PostSlug: &slug,
		WriteModel: request.WriteModel, Payload: payload,
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &generation.JobAlreadyInProgressError{ActiveID: active.ActiveID}
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
		ProgressDone: found.ProgressDone, ProgressTotal: found.ProgressTotal, Error: found.Error,
		PostSlug: postSlug, ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		CreatedAt: found.CreatedAt, UpdatedAt: found.UpdatedAt,
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
		ProgressDone: found.ProgressDone, ProgressTotal: found.ProgressTotal, Error: found.Error,
		PostSlug: postSlug, ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		CreatedAt: found.CreatedAt, UpdatedAt: found.UpdatedAt,
	}, nil
}

type experimentJobs struct{ queue *job.Queue }

func (a experimentJobs) EnqueueExperiment(ctx context.Context, request experiment.JobRequest) (string, error) {
	var postSlug *string
	if request.PostSlug != "" {
		value := request.PostSlug
		postSlug = &value
	}
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindModelExperiment, UserID: request.UserID, PostSlug: postSlug, Payload: []byte(request.ExperimentID),
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &experiment.JobAlreadyInProgressError{ActiveID: active.ActiveID}
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
	registry   *llm.Registry
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
	_, err := a.selections.SaveSelection(ctx, userID, provider.Stage(stage), llmRef(ref))
	return err
}

func (a experimentCatalog) Active(ctx context.Context, userID string, stage experiment.Stage) (experiment.ModelRef, bool, error) {
	selections, err := a.selections.GetSelections(ctx, userID)
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
		return experiment.Snapshot{Content: content, PromptVersion: generation.WriteExperimentPromptVersion}, mapSnapshotError(err)
	case experiment.StageObserve:
		content, err := a.generation.SnapshotObserveInput(ctx, request.UserID, request.PostSlug)
		return experiment.Snapshot{Content: content, PromptVersion: generation.ObserveExperimentPromptVersion}, mapSnapshotError(err)
	case experiment.StageAnalyze:
		content, err := a.voice.SnapshotAnalysisInput(ctx, request.UserID)
		return experiment.Snapshot{Content: content, PromptVersion: voice.AnalyzeExperimentPromptVersion}, err
	default:
		return experiment.Snapshot{}, experiment.ErrInvalidStage
	}
}

func (a experimentRunner) PrepareWrite(ctx context.Context, found experiment.Experiment, progress experiment.Progress) (experiment.Snapshot, error) {
	content, err := a.generation.PrepareWriteInput(ctx, found.InputSnapshot, generation.Progress(progress))
	return experiment.Snapshot{Content: content, PromptVersion: generation.WriteExperimentPromptVersion}, mapSnapshotError(err)
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
		return a.generation.ApplyWriteWinner(ctx, found.UserID, found.PostSlug, fromOutputPost(value), found.InputSnapshot)
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
		return a.voice.ApplyStyleguideWinner(ctx, found.UserID, styleguide)
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
	if err != nil && strings.Contains(err.Error(), "read photo") {
		return experiment.ErrSnapshotUnavailable
	}
	return err
}

var _ experiment.Runner = experimentRunner{}
