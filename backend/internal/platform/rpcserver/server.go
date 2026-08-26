// Package rpcserver is server plumbing only: it wires Connect handlers onto a
// net/http mux with h2c (cleartext HTTP/2) and CORS, plus a plain /health endpoint.
// It holds NO business logic — service implementations live in their own packages
// and are injected into New by the composition root (cmd/api).
package rpcserver

import (
	"encoding/json"
	"net/http"
	"time"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/config"
)

// maxRequestBytes caps a single unary request message. Raise it deliberately when
// image upload lands — until then a generous cap only bounds cheap large-POST abuse.
const maxRequestBytes = 256 << 10 // 256 KiB

// Server hardening timeouts: ReadTimeout bounds slow-body uploads (ReadHeaderTimeout
// covers headers only), WriteTimeout bounds slow readers, IdleTimeout reaps abandoned
// keep-alive connections. Unary handlers finish in milliseconds, so generous values
// bound abuse, not real traffic.
const (
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second
	idleTimeout  = 120 * time.Second
)

// New builds the fully-wired HTTP server: the given HealthService Connect handler
// plus a /health endpoint, wrapped in CORS and h2c. The caller owns the
// listen/shutdown lifecycle.
func New(cfg *config.Config, version string, healthSvc postpilotv1connect.HealthServiceHandler) *http.Server {
	mux := http.NewServeMux()

	opts := []connect.HandlerOption{
		connect.WithReadMaxBytes(maxRequestBytes),
	}
	healthPath, healthHandler := postpilotv1connect.NewHealthServiceHandler(healthSvc, opts...)
	mux.Handle(healthPath, healthHandler)

	// /health is mounted directly on the mux, so it bypasses the Connect stack — it is
	// what the platform (Caddy, uptime checks) probes, not an RPC.
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": version,
		})
	})

	// h2c wraps the outermost handler so the cleartext HTTP/2 upgrade applies to all
	// traffic (TLS is terminated at the edge Caddy). Read/WriteTimeout DO reach h2c
	// streams (x/net arms per-stream deadlines from the BaseConfig *http.Server) — only
	// IdleTimeout doesn't propagate, so the http2.Server mirrors it.
	root := h2c.NewHandler(withCORS(mux, cfg.CORSOrigin), &http2.Server{
		IdleTimeout: idleTimeout,
	})

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// withCORS allows the configured browser origin, using connect's recommended
// method/header sets plus Authorization (for the Bearer token auth will add).
func withCORS(h http.Handler, origin string) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins: []string{origin},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: append(connectcors.AllowedHeaders(), "Authorization"),
		ExposedHeaders: connectcors.ExposedHeaders(),
		MaxAge:         7200, // cache preflight (seconds)
	}).Handler(h)
}
