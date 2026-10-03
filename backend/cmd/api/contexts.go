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
	"github.com/postpilot/backend/internal/memory"
	memorystore "github.com/postpilot/backend/internal/memory/store"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
	"github.com/postpilot/backend/internal/quality"
	qualitystore "github.com/postpilot/backend/internal/quality/store"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/tosspay"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
	"github.com/postpilot/backend/internal/voucher"
	voucherstore "github.com/postpilot/backend/internal/voucher/store"
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
	// postFigures is the per-post credit figure (QUOTA-64), shared by the model list and the
	// estimator combos so both read one cached aggregate.
	postFigures *postFigures

	billingStore *billingstore.Store
	payments     billing.Provider
	billing      *billing.Service

	voucher *voucher.Service

	post    *post.Service
	quality *quality.Service

	clipStore         *clipstore.Store
	clipPorts         clipapp.Binder
	clipGuard         clipapp.Guard
	clip              *clipapp.Service
	clipSources       *clipapp.SourceService
	clipGeneration    *clipapp.GenerationService
	clipMediaRecovery *clipapp.MediaReconciler

	template   *template.Service
	guideline  *guideline.Service
	memory     *memory.Service
	provider   *provider.Service
	voice      *voice.Service
	generation *generation.Service

	experimentStore *experimentstore.Store
	experiment      *experiment.Service
}

type unavailableRateSource struct{}

func (unavailableRateSource) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 0, false, usage.ErrRateUnavailable
}

func buildContexts(ctx context.Context, p *platform) (*contexts, error) {
	cfg, handle, registry := p.cfg, p.db, p.registry
	if err := clip.DefaultMediaStageLimits(clipEnvironment(cfg)).Validate(); err != nil {
		return nil, fmt.Errorf("CLIP_MEDIA_* stage limits: require 0 < lease < wait <= stage <= 6h and 1..5 attempts: %w", err)
	}
	c := &contexts{platform: p}

	c.jobs = job.New(jobstore.New(handle.Writer, handle.Reader, jobKinds()), config.WorkerPollInterval, jobReporting{})
	c.jobs.AllowCancellation(jobCancellation{})

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
	var officialRate usage.RateSource = unavailableRateSource{}
	if cfg.EximAPIKey != "" {
		officialRate = fxrate.NewEximbank(cfg.EximAPIKey, &http.Client{Timeout: 5 * time.Second})
	}
	c.ledger = usage.NewService(
		usagestore.NewWithExports(handle.Writer, handle.Reader, func(tx *sql.Tx) usage.ExportWindowLedger {
			return usageExports{clipstore.NewTx(tx)}
		}), registry,
		int64(cfg.LLMMaxTokensDefault), anchors, approvedCeilingKinds()...,
	).WithModelGrades().WithOwnerCancellation(ownerCancellableKinds()...).WithRateSelector(usage.NewRateSelector(
		officialRate,
		usagestore.New(handle.Writer, handle.Reader),
	))
	c.postFigures = newPostFigures(c.ledger)
	c.billingStore = billingstore.New(handle.Writer, handle.Reader)
	c.billingStore.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return billingCredits{Service: usage.NewService(usagestore.NewTx(tx), nil, 0, anchors, approvedCeilingKinds()...), exports: clipstore.NewTx(tx)}
	})
	c.billingStore.SetPlansForTx(func(tx *sql.Tx) billing.Plans {
		return auth.NewService(authstore.NewTx(tx), cfg.SessionTTL, auth.Deps{Mailer: p.mailer})
	})
	c.billingStore.SetRefundBenefitsForTx(func(tx *sql.Tx) billing.RefundBenefits {
		return refundBenefits{credits: usagestore.NewTx(tx), exports: clipstore.NewTx(tx)}
	})
	c.billingStore.SetRefundBenefits(refundBenefits{
		credits: usagestore.New(handle.Writer, handle.Reader),
		exports: clipstore.New(handle.Writer, handle.Reader),
	})
	if cfg.BillingEnabled {
		c.payments = tosspay.New(cfg.TossSecretKey, http.DefaultClient)
	}
	c.billing = billing.NewService(
		c.billingStore, c.payments, nil, billingCredits{Service: c.ledger, exports: clipstore.New(handle.Writer, handle.Reader)}, c.auth, c.auth,
		billingMailer{mailer: p.mailer},
	).WithFixedKRW()
	voucherStore := voucherstore.New(handle.Writer, handle.Reader)
	voucherStore.SetCreditsForTx(func(tx *sql.Tx) voucher.Credits {
		return voucherCredits{usage.NewService(usagestore.NewTx(tx), nil, 0, anchors, approvedCeilingKinds()...)}
	})
	voucherStore.SetPaidCoverageForTx(func(tx *sql.Tx) voucher.PaidCoverage {
		return voucherPaidCoverage{billing: billing.NewService(billingstore.NewTx(tx), nil, nil, nil, nil, nil, nil)}
	})
	c.voucher = voucher.NewService(voucherStore, voucherCredits{c.ledger})
	c.metered = meteredRegistry{Registry: registry, ledger: c.ledger}
	// The curation surface's evidence, joined HERE rather than by a query inside the catalog:
	// usage_events belongs to the ledger, and a context reading another's tables is the one
	// rule ARCHITECTURE §2.2 exists to hold.
	p.catalog.SetReasoningSpend(catalogReasoningSpend{ledger: c.ledger, providerID: registry.ProviderID()})
	c.jobs.Admit(jobAdmission{ledger: c.ledger, registry: registry, plans: c.auth,
		jobs: jobstore.New(handle.Writer, handle.Reader, jobKinds())})
	c.clipPorts = clipTxPorts(c.ledger, registry, c.auth)
	c.clipGuard = clipapp.NewGuard(handle.Writer, c.clipPorts, jobstore.New(handle.Writer, handle.Writer, jobKinds()))

	// Post reads voice, guideline and experiment through adapters that resolve their
	// service after every context below exists (see adapters_post.go).
	c.post = post.NewService(
		poststore.New(handle.Writer, handle.Reader),
		p.bucket,
		post.Limits{
			PutTTL: cfg.PresignPutTTL, GetTTL: cfg.PresignGetTTL,
			MaxImageBytes: cfg.MaxImageBytes, MaxPhotos: cfg.MaxPhotosPerPost,
			MaxVideos: cfg.MaxVideosPerPost, MaxVideoBytes: cfg.MaxVideoBytes, MaxVideoSeconds: cfg.MaxVideoSeconds,
			AnswerLabelMax: cfg.Template.AskLabelMaxChars, AnswerValueMax: cfg.TemplateAskValueMaxChars,
		},
		post.Deps{
			Jobs:          postJobFinder{queue: c.jobs},
			Voices:        postVoices{app: c},
			Experiments:   postExperiments{app: c},
			ContentPurger: postExperiments{app: c},
			// Deleting a post drops the link its candidates named and nothing else.
			CandidateLinks: postCandidateLinks{app: c},
			// Deleting a post also drops the source links its memories named, and takes a
			// memory with it only when that post held the last one (MEM-17).
			MemoryLinks: postMemoryLinks{app: c},
			Fields:      blogFields{},
		},
	)
	// Quality reads only post, which exists by now, so its adapter holds the service directly.
	qualityStore := qualitystore.New(handle.Writer, handle.Reader)
	c.quality = quality.NewService(quality.Deps{
		Measurements: qualityStore, Posts: qualityPosts{service: c.post}, Now: time.Now,
	})
	c.clipStore = clipstore.New(handle.Writer, handle.Reader)
	c.clipSources = clipapp.NewSourceService(c.clipStore, p.bucket, clip.DefaultSourceLimits(clipEnvironment(cfg)))
	c.clip = clipapp.NewService(c.clipStore, clip.DefaultLimits(), c.clipSources, clipapp.NewFinalizer(handle.Writer, c.clipPorts, c.clipStore, clip.DefaultRenderConfig(clipEnvironment(cfg)), nil))
	c.clipMediaRecovery = clipapp.NewMediaReconciler(handle.Writer, c.clipPorts, c.clipStore, jobstore.New(handle.Writer, handle.Reader, jobKinds()), c.jobs, p.bucket, cfg.OrphanMinAge, nil)
	// External handoffs are reconciled before interruption/hold/source cleanup.
	if err := c.clipMediaRecovery.Reconcile(ctx); err != nil {
		return nil, fmt.Errorf("media handoff recovery: %w", err)
	}
	if n, err := c.jobs.SweepRunning(ctx); err != nil {
		return nil, fmt.Errorf("running job sweep: %w", err)
	} else if n > 0 {
		slog.Info("swept interrupted jobs", "count", n)
	}
	// After the admitter is attached, not with the other boot sweeps: an open hold can only
	// be settled through it, and a sweep that ran first would silently find nothing.
	if n, err := c.jobs.SweepOpenHolds(ctx); err != nil {
		return nil, fmt.Errorf("open credit hold sweep: %w", err)
	} else if n > 0 {
		slog.Info("settled holds left open by an interrupted finish", "count", n)
	}

	clipMedia, err := clipmedia.New(clip.DefaultMediaConfig(clipEnvironment(cfg)), nil)
	if err != nil {
		return nil, fmt.Errorf("clip media initialization: %w", err)
	}
	if err := clipMedia.CleanupStale(ctx, time.Now()); err != nil {
		return nil, fmt.Errorf("clip workspace cleanup: %w", err)
	}
	// Built ahead of the clip generation side, which records its revision requests as 영상 지침
	// candidates through it; its directories are wired below, once their contexts exist.
	c.guideline = guideline.NewService(
		guidelinestore.New(handle.Writer, handle.Reader),
		// The same 분야 directory post uses: one adapter over quality's list, not a second.
		blogFields{},
		guideline.Limits{TextMaxChars: cfg.GuidelineTextMaxChars, TitleMaxChars: cfg.GuidelineTitleMaxChars, MaxPerAccount: cfg.GuidelineMaxPerAccount},
		cfg.GuidelineCandidateMaxPending,
	)
	c.clipGeneration, err = newClipGeneration(ctx, cfg, c.clipStore, c.clip, c.clipSources, p.bucket, clipMedia, c.metered, c.auth, c.jobs, c.clipGuard, handle.Writer, c.clipPorts, clipGuidelineCandidates{service: c.guideline})
	if err != nil {
		return nil, fmt.Errorf("clip generation initialization: %w", err)
	}

	c.template = template.NewService(templatestore.New(handle.Writer, handle.Reader), templateLimits(cfg))
	// The template request (TMPL-58) is the one template surface that calls a model: the
	// metered registry, the post it may read as a sample, and the job it runs as.
	c.template.ConfigureRequests(
		templateModels{registry: c.metered}, templateSamples{service: c.post, photoRowMax: cfg.Template.PhotoRowMax},
		templateRequestJobs{queue: c.jobs}, template.RequestLimits(cfg.TemplateRequest),
	)
	c.template.ConfigureEstimate(templateEstimates{rates: c.ledger})
	c.post.SetTemplateDirectory(postTemplates{service: c.template})

	// Template names are a live projection and owned-id validation, never a stored column or
	// a SQL join: the guideline context asks the template context, through this adapter only —
	// and the clip context for a 영상 지침's video templates.
	c.guideline.SetTemplateDirectory(guidelineTemplates{service: c.template})
	c.guideline.SetVideoTemplateDirectory(guidelineVideoTemplates{service: c.clip})
	// The memory context stands alone: it reads no other context, and the only direction
	// anything crosses is the post-delete hook above, which hands it a slug.
	c.memory = memory.NewService(
		memorystore.New(handle.Writer, handle.Reader),
		memory.Limits{
			TextMaxChars:  cfg.MemoryTextMaxChars,
			TagsMax:       cfg.MemoryTagsMax,
			MaxPerAccount: cfg.MemoryMaxPerAccount,
			InjectMax:     cfg.MemoryInjectMax,
		},
	)

	c.provider = provider.NewService(
		providerstore.New(handle.Writer, handle.Reader), c.metered,
		providerCredits{ledger: c.ledger, plans: c.auth, budget: cfg.LLMCompletionBudget},
	).WithModelGrades().WithPostFigures(c.postFigures)
	// The extraction path: the account's analyze selection, the post it reads, and the
	// durable job it runs as. Everything else the memory context does needs none of them.
	c.memory.ConfigureExtraction(
		memoryModels{selections: c.provider, registry: c.metered},
		memoryPosts{service: c.post},
		memoryExtractions{queue: c.jobs},
	)

	c.voice = voice.NewService(
		voicestore.New(handle.Writer, handle.Reader),
		voiceModels{registry: c.metered},
		voiceJobs{queue: c.jobs, budget: cfg.LLMCompletionBudget},
	)
	c.voice.ConfigurePhotos(voiceObjects{bucket: p.bucket}, voice.PhotoLimits{
		PutTTL: cfg.PresignPutTTL, GetTTL: cfg.PresignGetTTL, MaxBytes: cfg.MaxImageBytes,
	})
	c.voice.ConfigurePosts(voicePosts{service: c.post})

	c.generation = generation.NewService(
		generationPosts{service: c.post},
		generationProfiles{service: c.voice},
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
			// Retrieval happens at enqueue and only for a post that opted in; the memory
			// context never learns that generation exists (MEM-19).
			Memories: generationMemories{service: c.memory},
			// A completed revision records what the user asked for as a candidate (GEN-42).
			// This adapter is the only place the two contexts meet in that direction, and
			// nothing crosses it but the account, the post and the user's own sentence.
			Candidates: generationCandidates{service: c.guideline},
			// The ticked rule texts are read at enqueue, for a write only; the quality context
			// never learns that generation exists (ARCH-7).
			QualityRules: generationQuality{service: c.quality},
			// The per-version generation snapshot (VOICE-29). Generation is the only context
			// that depends on both post and voice, so it is the only one that may join a
			// machine baseline to the profile version that produced it.
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
		experimentRunner{generation: c.generation},
		experimentPosts{service: c.post},
		cfg.ExperimentContentRetention,
	)
	if n, err := c.experiment.RecoverInterrupted(ctx); err != nil {
		return nil, fmt.Errorf("interrupted experiment recovery: %w", err)
	} else if n > 0 {
		slog.Info("recovered interrupted experiments", "count", n)
	}
	// The experiment reads the voice directory through a port adapted only here: a comparison
	// cannot retry in a deleted voice. No experiment holds a voice's deletion (VOICE-13).
	c.experiment.SetVoiceDirectory(experimentVoices{service: c.voice})
	c.experiment.SetVoiceReflection(experimentReflection{service: c.voice})
	return c, nil
}

// templateLimits is the template context's limits: the operator-set ceilings from config and the
// POST bounds its numbers seed.
func templateLimits(cfg *config.Config) template.Limits {
	return template.NewLimits(template.Ceilings(cfg.Template), postNumberBounds())
}

// postNumberBounds are the two generation numbers' bounds, taken from the POST option they seed
// rather than from a template limit of their own (TMPL-6): a template must not be able to
// store a number the post would refuse.
func postNumberBounds() template.NumberBounds {
	return template.NumberBounds{
		TargetLengthMin: post.TargetLengthMin, TargetLengthMax: post.TargetLengthMax, TagCountMin: post.TagCountRange.Min, TagCountMax: post.TagCountRange.Max,
	}
}
