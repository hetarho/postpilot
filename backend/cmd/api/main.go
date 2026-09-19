// Command api is the postpilot HTTP/RPC API server.
//
// This file is the composition root: the only place that wires configuration,
// infrastructure clients, and the server together. Every other package depends
// inward. The Connect server itself (mux, h2c, CORS, /health) is assembled in
// internal/platform/rpcserver.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/llm/openaicompat"
)

// adapters is the set of provider protocols this binary ships (PRD §6.4: 필요할 때
// 하나씩). This is the only place an adapter package is imported — the composition
// root injects them into the port, and nothing above the port sees them.
var adapters = map[string]llm.AdapterFactory{
	"openai_compatible": openaicompat.Factory,
}

const version = "0.0.1"

// main is four steps and no rule of its own (ARCH-40): the platform is loaded, the
// bounded contexts are constructed over it, the job kinds are bound to their owners,
// and the server runs until a signal or a listen failure ends it.
func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if runCommand(os.Args[1:]) {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	p, err := loadPlatform(ctx)
	if err != nil {
		fatal("platform", err)
	}
	defer p.db.Close()

	app, err := buildContexts(ctx, p)
	if err != nil {
		fatal("contexts", err)
	}
	registerJobs(app)
	if err := serve(ctx, app); err != nil {
		fatal("serve", err)
	}
}

// fatal is the one exit path: every boot failure happens before the listener exists,
// so the container never answers /health and the deploy gate rolls back (DEPLOY.md §2).
func fatal(step string, err error) {
	slog.Error(step+" failed", "err", err)
	os.Exit(1)
}
