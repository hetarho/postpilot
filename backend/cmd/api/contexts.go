package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	clipmedia "github.com/postpilot/backend/internal/clip/media"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/experiment"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
	"github.com/postpilot/backend/internal/fxrate"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/googleauth"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
	"github.com/postpilot/backend/internal/publishing"
	publishingstore "github.com/postpilot/backend/internal/publishing/store"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/tosspay"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// contexts is every bounded context the server runs, constructed once in dependency
// order. Adapters between contexts live in this package and nowhere else (ARCH-6).
type contexts struct {
	platform *platform

	jobs     *job.Queue
	auth     *auth.Service
	throttle *auth.Throttle
	ledger   *usage.Service
	metered  meteredRegistry

	billingStore *billingstore.Store
	payments     billing.Provider
	billing      *billing.Service

	post       *post.Service
	publishing *publishing.Service

	clipStore      *clipstore.Store
	clipPorts      clipapp.Binder
	clipGuard      clipapp.Guard
	clip           *clipapp.Service
	clipSources    *clipapp.SourceService
	clipGeneration *clipapp.GenerationService

	template   *template.Service
	guideline  *guideline.Service
	provider   *provider.Service
	voice      *voice.Service
	generation *generation.Service

	experimentStore *experimentstore.Store
	experiment      *experiment.Service
}

func buildContexts(ctx context.Context, p *platform) (*contexts, error) {
	cfg, handle, registry := p.cfg, p.db, p.registry
	c := &contexts{platform: p}

	c.jobs = job.New(jobstore.New(handle.Writer, handle.Reader, jobKinds()), config.WorkerPollInterval, jobReporting{})
	c.jobs.AllowCancellation(clipCancellation{})
	if n, err := c.jobs.SweepRunning(ctx); err != nil {
		return nil, fmt.Errorf("running job sweep: %w", err)
	} else if n > 0 {
		slog.Info("swept interrupted jobs", "count", n)
	}
	if n, err := c.jobs.SweepQueuedPersonalization(ctx); err != nil {
		return nil, fmt.Errorf("queued personalization sweep: %w", err)
	} else if n > 0 {
		slog.Info("held queued personalization for explicit retry", "count", n)
	}

	// auth ↔ ledger is a genuine cycle: the ledger's monthly windows anchor on the account
	// (and its subscription), and a tier upgrade owes the ledger credits (QUOTA-35). auth is
	// constructed first over closures that resolve the ledger at call time; buildContexts
	// assigns it a few lines below, before anything can call.
	authDeps := auth.Deps{
		Mailer:    p.mailer,
		WebOrigin: cfg.CORSOrigin,
		TopUp: func(ctx context.Context, userID string, credits int) error {
			return c.ledger.TopUpMonthlyLot(ctx, userID, credits)
		},
		Bootstraps: []auth.AccountBootstrap{
			func(ctx context.Context, userID string) error {
				return defaultVoiceBootstrap(ctx, handle, userID)
			},
			func(ctx context.Context, userID string) error {
				acting, err := c.auth.PlanOf(ctx, userID)
				if err != nil {
					return err
				}
				return c.ledger.EnsureMonthlyLot(ctx, userID, acting)
			},
		},
	}
	if cfg.GoogleClientID != "" {
		googleIdentity := googleauth.New(cfg.GoogleClientID, cfg.GoogleClientSecret, http.DefaultClient)
		googleIdentity.SetAllowedOrigin(cfg.CORSOrigin)
		authDeps.Google = googleIdentity
	}
	c.auth = auth.NewService(authstore.New(handle.Writer, handle.Reader), cfg.SessionTTL, authDeps)
	c.throttle = auth.NewThrottle()
	if n, err := c.auth.SweepExpired(ctx); err != nil {
		// Stale rows are harmless — they fail the expiry check on lookup anyway — so a
		// sweep failure is not worth refusing to serve over.
		slog.Warn("expired session sweep failed", "err", err)
	} else if n > 0 {
		slog.Info("swept expired sessions", "count", n)
	}

	// The ledger is built before any context that can spend: `metered` is the registry
	// every context is given from here on, so a call made anywhere lands on the ledger
	// without that context knowing the ledger exists.
	anchors := usageAnchors{auth: c.auth, late: c}
	c.ledger = usage.NewService(
		usagestore.New(handle.Writer, handle.Reader), registry,
		int64(cfg.LLMMaxTokensDefault), anchors, approvedCeilingKinds()...,
	)
	c.billingStore = billingstore.New(handle.Writer, handle.Reader)
	c.billingStore.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return usage.NewService(usagestore.NewTx(tx), nil, 0, anchors, approvedCeilingKinds()...)
	})
	c.billingStore.SetPlansForTx(func(tx *sql.Tx) billing.Plans {
		return auth.NewService(authstore.NewTx(tx), cfg.SessionTTL, auth.Deps{Mailer: p.mailer})
	})
	var exchangeRates billing.Rates
	if cfg.BillingEnabled {
		c.payments = tosspay.New(cfg.TossSecretKey, http.DefaultClient)
		exchangeRates = fxrate.NewEximbank(cfg.EximAPIKey, http.DefaultClient)
	}
	c.billing = billing.NewService(
		c.billingStore, c.payments, exchangeRates, c.ledger, c.auth, c.auth,
		billingMailer{mailer: p.mailer},
	)
	c.metered = meteredRegistry{Registry: registry, ledger: c.ledger}
	// The curation surface's evidence, joined HERE rather than by a query inside the catalog:
	// usage_events belongs to the ledger, and a context reading another's tables is the one
	// rule ARCHITECTURE §2.2 exists to hold.
	p.catalog.SetReasoningSpend(catalogReasoningSpend{ledger: c.ledger, providerID: registry.ProviderID()})
	c.jobs.Admit(jobAdmission{ledger: c.ledger, registry: registry, plans: c.auth})
	c.clipPorts = clipTxPorts(c.ledger, registry, c.auth)
	c.clipGuard = clipapp.NewGuard(handle.Writer, c.clipPorts, jobstore.New(handle.Writer, handle.Writer, jobKinds()))

	// After the admitter is attached, not with the other boot sweeps: an open hold can only
	// be settled through it, and a sweep that ran first would silently find nothing.
	if n, err := c.jobs.SweepOpenHolds(ctx); err != nil {
		return nil, fmt.Errorf("open credit hold sweep: %w", err)
	} else if n > 0 {
		slog.Info("settled holds left open by an interrupted finish", "count", n)
	}

	// post reads voice, guideline, experiment and publishing, and publishing reads post:
	// those adapters resolve their service through c at call time, after every context
	// below exists (see adapters_post.go).
	c.post = post.NewService(
		poststore.New(handle.Writer, handle.Reader),
		p.bucket,
		post.Limits{
			PutTTL: cfg.PresignPutTTL, GetTTL: cfg.PresignGetTTL,
			MaxImageBytes: cfg.MaxImageBytes, MaxPhotos: cfg.MaxPhotosPerPost,
			MaxVideos: cfg.MaxVideosPerPost, MaxVideoBytes: cfg.MaxVideoBytes, MaxVideoSeconds: cfg.MaxVideoSeconds,
			AnswerLabelMax: cfg.TemplateAskLabelMaxChars, AnswerValueMax: cfg.TemplateAskValueMaxChars,
		},
		post.Deps{
			Jobs:   postJobFinder{queue: c.jobs},
			Voices: postVoices{app: c},
			// The post context refuses to delete a post whose publication is still in
			// flight; it learns that only through this adapter, never by importing
			// internal/publishing.
			LivePublish:   postPublications{app: c},
			Experiments:   postExperiments{app: c},
			ContentPurger: postExperiments{app: c},
			// Deleting a post drops the link its candidates named and nothing else.
			CandidateLinks: postCandidateLinks{app: c},
		},
	)
	c.publishing = publishing.NewService(
		publishingstore.New(handle.Writer, handle.Reader),
		publishingPosts{service: c.post},
		p.bucket,
		publishing.Config{
			PairingTTL: cfg.PublishPairingTTL, MaxPendingPairings: cfg.PublishMaxPendingPairings,
			LeaseTTL: cfg.PublishLeaseTTL, AssetURLTTL: cfg.PublishAssetURLTTL,
			AgentHeartbeatInterval: cfg.PublishAgentHeartbeatInterval,
		},
	)
	if requeued, unknown, err := c.publishing.RecoverExpired(ctx); err != nil {
		return nil, fmt.Errorf("publishing recovery: %w", err)
	} else if requeued > 0 || unknown > 0 {
		slog.Info("publishing jobs recovered", "requeued", requeued, "outcome_unknown", unknown)
	}

	c.clipStore = clipstore.New(handle.Writer, handle.Reader)
	c.clipSources = clipapp.NewSourceService(c.clipStore, p.bucket, clip.DefaultSourceLimits(clipEnvironment(cfg)))
	c.clip = clipapp.NewService(c.clipStore, clip.DefaultLimits(), c.clipSources, clipapp.NewFinalizer(handle.Writer, c.clipPorts, c.clipStore, clip.DefaultRenderConfig(clipEnvironment(cfg)), nil))
	clipMedia, err := clipmedia.New(clip.DefaultMediaConfig(clipEnvironment(cfg)), nil)
	if err != nil {
		return nil, fmt.Errorf("clip media initialization: %w", err)
	}
	if err := clipMedia.CleanupStale(ctx, time.Now()); err != nil {
		return nil, fmt.Errorf("clip workspace cleanup: %w", err)
	}
	c.clipGeneration, err = newClipGeneration(ctx, cfg, c.clipStore, c.clip, c.clipSources, p.bucket, clipMedia, c.metered, c.jobs, c.clipGuard, handle.Writer, c.clipPorts)
	if err != nil {
		return nil, fmt.Errorf("clip generation initialization: %w", err)
	}

	c.template = template.NewService(
		templatestore.New(handle.Writer, handle.Reader),
		template.Limits{
			NameMaxChars: cfg.TemplateNameMaxChars, DescriptionMaxChars: cfg.TemplateDescriptionMaxChars,
			BodyMaxChars: cfg.TemplateBodyMaxChars, MaxPerAccount: cfg.TemplateMaxPerAccount,
			MaxRepeatExpansion: cfg.TemplateMaxRepeatExpansion,
			PhotoRowMax:        cfg.TemplatePhotoRowMax,
			AskLabelMaxChars:   cfg.TemplateAskLabelMaxChars,
			AskMaxPerBody:      cfg.TemplateAskMaxPerBody,
			// The two generation numbers are bounded by the POST option they seed, not by a
			// template limit of their own (TEMPLATE-6): a template must not be able to store a
			// number the post would refuse.
			TargetLengthMin: 1,
			TagCountMin:     post.TagCountRange.Min,
			TagCountMax:     post.TagCountRange.Max,
		},
	)
	c.post.SetTemplateDirectory(postTemplates{service: c.template})

	c.guideline = guideline.NewService(
		guidelinestore.New(handle.Writer, handle.Reader),
		guideline.Limits{TextMaxChars: cfg.GuidelineTextMaxChars, MaxPerAccount: cfg.GuidelineMaxPerAccount},
		cfg.GuidelineCandidateMaxPending,
	)
	// Template names are a live projection and owned-id validation, never a stored column or
	// a SQL join: the guideline context asks the template context, through this adapter only.
	c.guideline.SetTemplateDirectory(guidelineTemplates{service: c.template})

	c.provider = provider.NewService(
		providerstore.New(handle.Writer, handle.Reader), c.metered,
		providerCredits{ledger: c.ledger, plans: c.auth, budget: cfg.LLMCompletionBudget},
	)
	c.voice = voice.NewService(
		voicestore.New(handle.Writer, handle.Reader),
		voiceModels{selections: c.provider, registry: c.metered, plans: c.auth},
		voiceJobs{queue: c.jobs},
	)
	c.voice.ConfigurePersonalization(voicePosts{service: c.post}, voice.PersonalizationThresholds())

	c.generation = generation.NewService(
		generationPosts{service: c.post},
		generationProfiles{service: c.voice},
		generationRules{service: c.voice},
		generationModels{registry: c.metered},
		generationImages{bucket: p.bucket},
		generationJobs{queue: c.jobs, budget: cfg.LLMCompletionBudget},
		cfg.ObserveBatchSize,
		generation.DefaultReasoningPolicy(),
		// The budget policy is passed whole rather than as numbers: the stages ask their
		// owner what their work needs, and this context holds no cap of its own.
		cfg.LLMCompletionBudget,
		generation.Deps{
			// The experiment context is constructed after generation; the adapter resolves
			// it at call time (see postExperiments).
			Experiments: postExperiments{app: c},
			// Generation reads a brief and the 지침 only at enqueue, to freeze them; the
			// template and guideline contexts never learn that generation exists.
			Templates:  generationTemplates{service: c.template},
			Guidelines: generationGuidelines{service: c.guideline},
			// A completed revision records what the user asked for as a candidate (change 26).
			// This adapter is the only place the two contexts meet in that direction, and
			// nothing crosses it but the account, the post and the user's own sentence.
			Candidates: generationCandidates{service: c.guideline},
			// The per-version generation snapshot (change 16). Generation is the only context
			// that depends on both post and voice, so it is the only one that may join a
			// machine baseline to the profile version that produced it.
			Samples: generationVersionSamples{service: c.voice},
			// A video reaches a model as a LINK, minted per call and living exactly as long
			// as a browser view URL does — the bytes never enter this process (VIDEO-10).
			Videos: p.bucket, VideoURLTTL: cfg.PresignGetTTL,
		},
	)

	c.experimentStore = experimentstore.New(handle.Writer, handle.Reader)
	c.experiment = experiment.NewService(
		c.experimentStore,
		experimentCatalog{selections: c.provider, registry: c.metered, plans: c.auth},
		experimentJobs{queue: c.jobs},
		experimentRunner{generation: c.generation, voice: c.voice},
		cfg.ExperimentContentRetention,
	)
	if n, err := c.experiment.RecoverInterrupted(ctx); err != nil {
		return nil, fmt.Errorf("interrupted experiment recovery: %w", err)
	} else if n > 0 {
		slog.Info("recovered interrupted experiments", "count", n)
	}
	// Voice and experiment guard each other through ports adapted only here: a voice with a
	// publishable experiment cannot be deleted, and an experiment cannot start or retry in a
	// deleted voice.
	c.voice.SetExperimentGuard(voiceExperiments{service: c.experiment})
	c.experiment.SetVoiceDirectory(experimentVoices{service: c.voice})
	return c, nil
}
