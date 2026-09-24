package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/mail"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/modelcatalog/openrouter"
	modelcatalogstore "github.com/postpilot/backend/internal/modelcatalog/store"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/storage"
)

// platform is everything with no business meaning that the contexts are built over:
// configuration, the database, the model registry and catalog, object storage and the
// mail edge (ARCH-6).
type platform struct {
	cfg      *config.Config
	db       *db.DB
	catalog  *modelcatalog.Service
	registry *llm.Registry
	bucket   *storage.Bucket
	mailer   auth.Mailer
}

func loadPlatform(ctx context.Context) (*platform, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}
	handle, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("database open %s: %w", cfg.DBPath, err)
	}
	p := &platform{cfg: cfg, db: handle}
	if err := p.load(ctx); err != nil {
		handle.Close()
		return nil, err
	}
	return p, nil
}

func (p *platform) load(ctx context.Context) error {
	cfg := p.cfg
	if err := cfg.RequireMediaControl(); err != nil {
		return fmt.Errorf("media control config invalid: %w", err)
	}
	// Migrations run before the listener exists, and a failure exits non-zero ([I7]).
	// That ordering is the whole rollback mechanism: the container never answers
	// /health, so the deploy gate restores the previous image (DEPLOY.md §2).
	if err := db.Migrate(ctx, p.db.Writer); err != nil {
		return fmt.Errorf("migration: %w", err)
	}

	// The registry is loaded after the database because its models are curated rows now,
	// not yaml entries (plan 18). What the yaml still decides — how to reach the provider —
	// keeps the same posture as a migration: a file that does not validate must not serve,
	// and this still runs before the listener exists, so the deploy's /health gate rolls
	// back. A missing API key is NOT that: the models come up disabled instead.
	//
	// An empty catalog is a valid state. A fresh install has curated nothing, and the right
	// answer is an empty dropdown and a trip to /admin/models, not a refused boot.
	p.catalog = modelcatalog.NewService(modelcatalogstore.New(p.db.Writer, p.db.Reader))
	if err := p.catalog.Reload(ctx); err != nil {
		return fmt.Errorf("model catalog load: %w", err)
	}
	registry, err := llm.Load(cfg.ProvidersConfig, os.Getenv, adapters, p.catalog, llm.Options{
		Timeout:              cfg.LLMStageTimeout,
		MaxTokens:            cfg.LLMMaxTokensDefault,
		EndpointCacheTTL:     cfg.CatalogTTL,
		EndpointFetchTimeout: cfg.CatalogFetchTimeout,
	})
	if err != nil {
		return fmt.Errorf("providers config invalid: %w", err)
	}
	p.registry = registry
	// The upstream catalog lives at the registered endpoint, so its address is configured in
	// exactly one place. Attached after Load because that is where the address comes from;
	// boot itself never calls it.
	p.catalog.SetUpstream(openrouter.New(registry.BaseURL(), cfg.CatalogFetchTimeout, cfg.CatalogTTL))
	for _, m := range registry.Models() {
		if m.Disabled {
			slog.Warn("model disabled", "model", m.Ref.String(), "reason", m.DisabledReason)
		}
	}

	// Checked here rather than in config.Load: `api adduser` must work on a fresh box
	// before a bucket exists (it returns before this step). This still runs before the
	// listener, so a missing value keeps /health dark and the deploy rolls back.
	if err := cfg.RequireObjectStorage(); err != nil {
		return fmt.Errorf("object storage config invalid: %w", err)
	}
	p.bucket, err = storage.New(ctx, storage.Config{
		Endpoint:        cfg.R2Endpoint,
		PublicEndpoint:  cfg.R2PublicEndpoint,
		MediaEndpoint:   cfg.MediaStorageEndpoint,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket:          cfg.R2Bucket,
		MaxReadBytes:    cfg.MaxImageBytes,
	})
	if err != nil {
		return fmt.Errorf("object storage setup: %w", err)
	}

	p.mailer = mail.NewLog()
	if cfg.MailDriver == "resend" {
		p.mailer = mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom, http.DefaultClient)
	}
	return nil
}
