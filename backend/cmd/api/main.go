// Command api is the postpilot HTTP/RPC API server.
//
// This file is the composition root: the only place that wires configuration,
// infrastructure clients, and the server together. Every other package depends
// inward. The Connect server itself (mux, h2c, CORS, /health) is assembled in
// internal/platform/rpcserver.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/auth/provision"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/health"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/post"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/storage"
)

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
	)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "err", err)
	}
}
