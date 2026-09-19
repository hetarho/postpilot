package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	"github.com/postpilot/backend/internal/billing"
	billingrpc "github.com/postpilot/backend/internal/billing/rpc"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"github.com/postpilot/backend/internal/experiment"
	experimentrpc "github.com/postpilot/backend/internal/experiment/rpc"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	generationrpc "github.com/postpilot/backend/internal/generation/rpc"
	guidelinerpc "github.com/postpilot/backend/internal/guideline/rpc"
	modelcatalogrpc "github.com/postpilot/backend/internal/modelcatalog/rpc"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/health"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
	poststore "github.com/postpilot/backend/internal/post/store"
	providerrpc "github.com/postpilot/backend/internal/provider/rpc"
	"github.com/postpilot/backend/internal/publishing"
	publishingrpc "github.com/postpilot/backend/internal/publishing/rpc"
	templaterpc "github.com/postpilot/backend/internal/template/rpc"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
)

// serve assembles the Connect server over the contexts, starts the sweepers and the
// workers, and blocks until a signal or a listen failure. A listen failure is returned so
// main exits non-zero: an orchestrator would read a clean exit as success and never
// restart us.
func serve(ctx context.Context, c *contexts) error {
	p := c.platform
	cfg, handle := p.cfg, p.db
	server := rpcserver.New(cfg, version, rpcserver.Options{
		Interceptors: []connect.Interceptor{authrpc.NewInterceptor(c.auth, c.throttle, cfg.ClientIPHeader), publishingrpc.NewAgentInterceptor(c.publishing)},
		Handlers:     handlers(c),
		Routes: map[string]http.Handler{
			// These plain routes bypass the Connect interceptors, so the throttle the
			// authenticated public writes get has to be put on this one here. Composition is
			// also the only place it can go: billing/rpc importing auth is the wrong
			// direction (ARCH-7).
			"/webhooks/toss": throttledRoute(
				c.throttle, auth.ThrottleWebhook, cfg.ClientIPHeader,
				billingrpc.NewWebhookHandler(c.payments, c.billingStore),
			),
		},
	})

	// The sweep starts with the server and stops with it. It is deliberately not run at
	// boot: a restart loop would turn every crash into a full bucket listing, and
	// nothing it collects is urgent.
	sweeper := post.NewSweeper(
		poststore.New(handle.Writer, handle.Reader),
		p.bucket,
		cfg.OrphanMinAge,
	)
	go sweeper.Run(ctx, cfg.OrphanSweepInterval)
	go c.clipGeneration.RunSweep(ctx, cfg.ClipSourceSweepInterval)
	go experiment.NewSweeper(c.experimentStore).Run(ctx, cfg.ExperimentSweepInterval)
	go publishing.NewSweeper(c.publishing, cfg.PublishOrphanMinAge, cfg.PublishLeaseTTL).Run(ctx, cfg.PublishOrphanSweepInterval)
	go func() {
		ticker := time.NewTicker(config.ThrottleSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				c.throttle.Sweep(now)
			}
		}
	}()
	if cfg.BillingEnabled {
		go runBillingWorker(ctx, c.billing)
	}

	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	for range config.WorkerConcurrency {
		go c.jobs.Run(workerCtx)
	}

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
		return fmt.Errorf("listen: %w", err)
	}
	cancelWorkers()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "err", err)
	}
	return nil
}

// handlers is every Connect service the server exposes, each over its own context.
func handlers(c *contexts) []rpcserver.Registrar {
	cfg := c.platform.cfg
	catalog := c.platform.catalog
	return []rpcserver.Registrar{
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewHealthServiceHandler(health.NewHandler(version), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewAuthServiceHandler(authrpc.NewHandler(c.auth, cfg.SessionTTL), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewPlanServiceHandler(planrpc.NewHandler(planBalance{ledger: c.ledger}, estimatorCombos{catalog: catalog}), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewBillingServiceHandler(billingrpc.NewHandler(c.billing), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewAdminServiceHandler(authrpc.NewAdminHandler(c.auth, comboAssigner{catalog: catalog}), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewPostServiceHandler(postrpc.NewHandler(c.post), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewProviderServiceHandler(providerrpc.NewHandler(c.provider), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewModelCatalogServiceHandler(modelcatalogrpc.NewHandler(catalog), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewTemplateServiceHandler(templaterpc.NewHandler(c.template), opts...)
		},
		// One clip handler, five services (ARCH-41): the rpcs are grouped by family so a task
		// stream editing one family edits one proto file, while the context behind them is
		// still one — splitting the handler would only split the services it holds.
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewClipTemplateServiceHandler(clipHandler(c), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewClipSourceServiceHandler(clipHandler(c), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewClipGenerationServiceHandler(clipHandler(c), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewClipPlanServiceHandler(clipHandler(c), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewClipRenderServiceHandler(clipHandler(c), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewGuidelineServiceHandler(guidelinerpc.NewHandler(c.guideline), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewGenerationServiceHandler(generationrpc.NewHandler(c.generation), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewVoiceServiceHandler(voicerpc.NewHandler(c.voice), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewVoiceLearningServiceHandler(voicerpc.NewLearningHandler(c.voice), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewVoiceValidationServiceHandler(voicerpc.NewValidationHandler(c.voice), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewModelExperimentServiceHandler(experimentrpc.NewHandler(c.experiment), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewPublishingServiceHandler(publishingrpc.NewUserHandler(c.publishing), opts...)
		},
		func(opts ...connect.HandlerOption) (string, http.Handler) {
			return postpilotv1connect.NewPublishingAgentServiceHandler(publishingrpc.NewAgentHandler(c.publishing), opts...)
		},
	}
}

func throttledRoute(throttle *auth.Throttle, class, clientIPHeader string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		retryAt, allowed := throttle.Allow(class, auth.ClientIP(r.Header, r.RemoteAddr, clientIPHeader), time.Now())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(max(int(time.Until(retryAt).Seconds()), 1)))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func runBillingWorker(ctx context.Context, service *billing.Service) {
	runBillingPasses(ctx, config.BillingTickInterval, func(now time.Time) error {
		return service.RunDue(ctx, now)
	})
}

// runBillingPasses catches up once and then once per tick, inside the caller's goroutine.
//
// The catch-up pass is here rather than in composition on purpose: a pass charges cards,
// looks up rates and sends mail per due account on default HTTP timeouts, so a backlog after
// an outage would keep the listener — and `/health` with it — shut long enough for the
// health-gated rollout to roll the release back (ARCH-32). A pass that fails or runs long
// costs its own tick and nothing else.
func runBillingPasses(ctx context.Context, interval time.Duration, pass func(time.Time) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	if err := pass(time.Now()); err != nil {
		slog.Error("billing renewal pass failed", "err", err, "boot", true)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := pass(now); err != nil {
				slog.Error("billing renewal pass failed", "err", err)
			}
		}
	}
}

// estimatorCombos hands the plan edge the priced combos the catalog owns, mapping the
// catalog's shape onto the edge's own so neither imports the other (ARCHITECTURE §2.2).

// clipHandler builds the clip context's Connect edge. It answers for all five clip services.
func clipHandler(c *contexts) *cliprpc.Handler {
	return cliprpc.NewHandler(c.clip).WithSources(c.clipSources).WithGeneration(c.clipGeneration, c.jobs)
}
